package main

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	okf "github.com/xSAVIKx/okf-skills/okf-go"
)

// entity is a table or view discovered from sqlite_master, with its defining SQL.
type entity struct {
	Name string
	Kind string // "table" | "view"
	SQL  string // CREATE statement (the view definition for views)
}

// listEntities returns the tables and views in the database (excluding sqlite
// internals), ordered by name for deterministic output, optionally filtered.
func listEntities(db *sql.DB, filter map[string]bool) ([]entity, error) {
	rows, err := db.Query("SELECT name, type, COALESCE(sql, '') FROM sqlite_master WHERE type IN ('table','view') AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ents []entity
	for rows.Next() {
		var e entity
		if err := rows.Scan(&e.Name, &e.Kind, &e.SQL); err != nil {
			return nil, err
		}
		if filter == nil || filter[e.Name] {
			ents = append(ents, e)
		}
	}
	return ents, rows.Err()
}

// getIndexesAndConstraints reads PRAGMA index_list / index_info for a table and
// returns its indexes plus the UNIQUE constraints derived from unique indexes.
func getIndexesAndConstraints(db *sql.DB, table string) ([]okf.Index, []okf.Constraint, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA index_list(`%s`)", table))
	if err != nil {
		return nil, nil, err
	}
	type il struct {
		name   string
		unique bool
		origin string
	}
	var lists []il
	for rows.Next() {
		var seq, unique, partial int
		var name, origin string
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			rows.Close()
			return nil, nil, err
		}
		lists = append(lists, il{name: name, unique: unique == 1, origin: origin})
	}
	rows.Close()

	var indexes []okf.Index
	var cons []okf.Constraint
	for _, it := range lists {
		cols, err := indexColumns(db, it.name)
		if err != nil {
			return nil, nil, err
		}
		indexes = append(indexes, okf.Index{Name: it.name, Columns: cols, Unique: it.unique})
		if it.origin == "u" { // index created by a UNIQUE constraint
			cons = append(cons, okf.Constraint{Name: it.name, Type: "UNIQUE", Definition: strings.Join(cols, ", ")})
		}
	}
	return indexes, cons, nil
}

// indexColumns returns the column names of an index in order.
func indexColumns(db *sql.DB, index string) ([]string, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA index_info(`%s`)", index))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var seqno, cid int
		var name sql.NullString
		if err := rows.Scan(&seqno, &cid, &name); err != nil {
			return nil, err
		}
		if name.Valid {
			cols = append(cols, name.String)
		}
	}
	return cols, rows.Err()
}

// parseCheckConstraints extracts CHECK(...) constraints from a table's CREATE SQL.
// SQLite does not expose check constraints via PRAGMA, so we parse them from the
// stored DDL with balanced-parenthesis extraction. Pure and deterministic.
func parseCheckConstraints(createSQL string) []okf.Constraint {
	var cons []okf.Constraint
	lower := strings.ToLower(createSQL)
	from := 0
	n := 0
	for {
		idx := strings.Index(lower[from:], "check")
		if idx < 0 {
			break
		}
		pos := from + idx
		// Advance to the opening paren after "check".
		p := pos + len("check")
		for p < len(createSQL) && (createSQL[p] == ' ' || createSQL[p] == '\t' || createSQL[p] == '\n') {
			p++
		}
		if p >= len(createSQL) || createSQL[p] != '(' {
			from = pos + len("check")
			continue
		}
		expr, end, ok := extractBalanced(createSQL, p)
		if !ok {
			break
		}
		n++
		cons = append(cons, okf.Constraint{
			Name:       fmt.Sprintf("check_%d", n),
			Type:       "CHECK",
			Definition: strings.TrimSpace(expr),
		})
		from = end
	}
	return cons
}

// extractBalanced returns the content between the parenthesis at start and its
// matching close paren, the index just past the close, and whether it balanced.
func extractBalanced(s string, start int) (string, int, bool) {
	if start >= len(s) || s[start] != '(' {
		return "", start, false
	}
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return s[start+1 : i], i + 1, true
			}
		}
	}
	return "", len(s), false
}

var freshnessTypeHint = regexp.MustCompile(`(?i)DATE|TIME|TIMESTAMP`)
var freshnessNameHint = regexp.MustCompile(`(?i)(^|_)(date|time|timestamp)|_at$|_on$`)

// detectFreshnessColumn picks a timestamp-like column to drive freshness stats,
// preferring a date/time *type* and falling back to a date/time-ish *name*. Pure.
func detectFreshnessColumn(cols []Column) string {
	for _, c := range cols {
		if freshnessTypeHint.MatchString(c.Type) {
			return c.Name
		}
	}
	for _, c := range cols {
		if freshnessNameHint.MatchString(c.Name) {
			return c.Name
		}
	}
	return ""
}

// getTableStats computes a row count and (if a timestamp column is detected) a
// freshness window. Gated by the connector's --stats flag.
func getTableStats(db *sql.DB, table string, cols []Column) (okf.TableStats, error) {
	var s okf.TableStats
	var cnt int64
	if err := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM `%s`", table)).Scan(&cnt); err != nil {
		return s, err
	}
	s.RowCount = cnt
	s.HasRowCount = true

	if col := detectFreshnessColumn(cols); col != "" {
		var mn, mx sql.NullString
		q := fmt.Sprintf("SELECT CAST(MIN(`%[1]s`) AS TEXT), CAST(MAX(`%[1]s`) AS TEXT) FROM `%[2]s`", col, table)
		if err := db.QueryRow(q).Scan(&mn, &mx); err == nil {
			s.FreshnessColumn = col
			s.Earliest = mn.String
			s.Latest = mx.String
		}
	}
	return s, nil
}

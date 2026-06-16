package main

import (
	"database/sql"
	"fmt"
	"regexp"
	"sort"
	"strings"

	okf "github.com/xSAVIKx/okf-skills/okf-go"
)

// entity is a table or view discovered from information_schema, with its comment.
type entity struct {
	Name    string
	Comment string
	IsView  bool
}

// listEntities returns the base tables and views in a PostgreSQL schema, ordered
// by name for deterministic output, optionally filtered. The table/view comment
// is read from obj_description so the comment-based description is preserved.
func listEntities(db *sql.DB, schema string, filter map[string]bool) ([]entity, error) {
	rows, err := db.Query(`
		SELECT
			t.table_name,
			t.table_type,
			COALESCE(obj_description(c.oid, 'pg_class'), '') AS table_comment
		FROM information_schema.tables t
		JOIN pg_namespace n ON n.nspname = t.table_schema
		JOIN pg_class c ON c.relname = t.table_name AND c.relnamespace = n.oid
		WHERE t.table_schema = $1 AND t.table_type IN ('BASE TABLE', 'VIEW')
		ORDER BY t.table_name`, schema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ents []entity
	for rows.Next() {
		var name, tableType, comment string
		if err := rows.Scan(&name, &tableType, &comment); err != nil {
			return nil, err
		}
		if filter != nil && !filter[name] {
			continue
		}
		ents = append(ents, entity{
			Name:    name,
			Comment: strings.TrimSpace(comment),
			IsView:  tableType == "VIEW",
		})
	}
	return ents, rows.Err()
}

// getViewDefinition returns the SQL definition of a view, or "" when none.
func getViewDefinition(db *sql.DB, schema, view string) (string, error) {
	var def sql.NullString
	err := db.QueryRow(`
		SELECT view_definition
		FROM information_schema.views
		WHERE table_schema = $1 AND table_name = $2`, schema, view).Scan(&def)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(def.String), nil
}

// getIndexes reads a table's indexes from the pg_index/pg_class catalog, grouping
// the index columns in their declared order. Purely a catalog read.
func getIndexes(db *sql.DB, schema, table string) ([]okf.Index, error) {
	rows, err := db.Query(`
		SELECT i.relname AS index_name, ix.indisunique, a.attname
		FROM pg_class t
		JOIN pg_index ix ON t.oid = ix.indrelid
		JOIN pg_class i ON i.oid = ix.indexrelid
		JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY(ix.indkey)
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE t.relname = $1 AND n.nspname = $2
		ORDER BY i.relname, array_position(ix.indkey, a.attnum)`, table, schema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type idxAcc struct {
		unique bool
		cols   []string
	}
	order := []string{}
	acc := map[string]*idxAcc{}
	for rows.Next() {
		var name, col string
		var unique bool
		if err := rows.Scan(&name, &unique, &col); err != nil {
			return nil, err
		}
		a, ok := acc[name]
		if !ok {
			a = &idxAcc{unique: unique}
			acc[name] = a
			order = append(order, name)
		}
		a.cols = append(a.cols, col)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var indexes []okf.Index
	for _, name := range order {
		a := acc[name]
		indexes = append(indexes, okf.Index{Name: name, Columns: a.cols, Unique: a.unique})
	}
	return indexes, nil
}

// getConstraints reads UNIQUE and CHECK constraints for a table from
// information_schema. Postgres' implicit NOT NULL check constraints are filtered
// out so only meaningful, user-declared checks surface.
func getConstraints(db *sql.DB, schema, table string) ([]okf.Constraint, error) {
	var cons []okf.Constraint

	// UNIQUE constraints: group key columns in ordinal order per constraint.
	uRows, err := db.Query(`
		SELECT tc.constraint_name, kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		  ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
		WHERE tc.constraint_type = 'UNIQUE' AND tc.table_schema = $1 AND tc.table_name = $2
		ORDER BY kcu.ordinal_position`, schema, table)
	if err != nil {
		return nil, err
	}
	uOrder := []string{}
	uCols := map[string][]string{}
	for uRows.Next() {
		var name, col string
		if err := uRows.Scan(&name, &col); err != nil {
			uRows.Close()
			return nil, err
		}
		if _, ok := uCols[name]; !ok {
			uOrder = append(uOrder, name)
		}
		uCols[name] = append(uCols[name], col)
	}
	if err := uRows.Err(); err != nil {
		uRows.Close()
		return nil, err
	}
	uRows.Close()
	for _, name := range uOrder {
		cons = append(cons, okf.Constraint{
			Name:       name,
			Type:       "UNIQUE",
			Definition: strings.Join(uCols[name], ", "),
		})
	}

	// CHECK constraints: resolve via table_constraints (reliable per-table) and
	// skip Postgres' implicit NOT NULL checks.
	cRows, err := db.Query(`
		SELECT cc.constraint_name, cc.check_clause
		FROM information_schema.table_constraints tc
		JOIN information_schema.check_constraints cc
		  ON tc.constraint_name = cc.constraint_name AND tc.constraint_schema = cc.constraint_schema
		WHERE tc.constraint_type = 'CHECK' AND tc.table_schema = $1 AND tc.table_name = $2`, schema, table)
	if err != nil {
		return nil, err
	}
	defer cRows.Close()
	for cRows.Next() {
		var name, clause string
		if err := cRows.Scan(&name, &clause); err != nil {
			return nil, err
		}
		if strings.Contains(strings.ToUpper(clause), "IS NOT NULL") {
			continue
		}
		cons = append(cons, okf.Constraint{
			Name:       name,
			Type:       "CHECK",
			Definition: strings.TrimSpace(clause),
		})
	}
	if err := cRows.Err(); err != nil {
		return nil, err
	}

	sort.SliceStable(cons, func(i, j int) bool { return cons[i].Name < cons[j].Name })
	return cons, nil
}

var freshnessTypeHint = regexp.MustCompile(`(?i)DATE|TIME|TIMESTAMP`)
var freshnessNameHint = regexp.MustCompile(`(?i)(^|_)(date|time|timestamp)|_at$|_on$`)

// detectFreshnessColumn picks a timestamp-like column to drive freshness stats,
// preferring a date/time *type* and falling back to a date/time-ish *name*. Pure.
func detectFreshnessColumn(cols []ColumnSpec) string {
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
func getTableStats(db *sql.DB, schema, table string, cols []ColumnSpec) (okf.TableStats, error) {
	var s okf.TableStats
	var cnt int64
	q := fmt.Sprintf("SELECT COUNT(*) FROM %s.%s", quoteIdent(schema), quoteIdent(table))
	if err := db.QueryRow(q).Scan(&cnt); err != nil {
		return s, err
	}
	s.RowCount = cnt
	s.HasRowCount = true

	if col := detectFreshnessColumn(cols); col != "" {
		var mn, mx sql.NullString
		fq := fmt.Sprintf("SELECT MIN(%[1]s)::text, MAX(%[1]s)::text FROM %[2]s.%[3]s",
			quoteIdent(col), quoteIdent(schema), quoteIdent(table))
		if err := db.QueryRow(fq).Scan(&mn, &mx); err == nil {
			s.FreshnessColumn = col
			s.Earliest = mn.String
			s.Latest = mx.String
		}
	}
	return s, nil
}

package main

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	okf "github.com/xSAVIKx/okf-skills/okf-go"
)

// getViewDefinition returns the SQL body of a view from information_schema.VIEWS.
func getViewDefinition(db *sql.DB, schema, view string) (string, error) {
	var def sql.NullString
	err := db.QueryRow(`
		SELECT VIEW_DEFINITION
		FROM information_schema.VIEWS
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?`, schema, view).Scan(&def)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return def.String, nil
}

// getIndexes reads index metadata from information_schema.STATISTICS and groups
// the per-column rows into okf.Index entries with columns in SEQ_IN_INDEX order.
func getIndexes(db *sql.DB, schema, table string) ([]okf.Index, error) {
	rows, err := db.Query(`
		SELECT INDEX_NAME, NON_UNIQUE, COLUMN_NAME, SEQ_IN_INDEX
		FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY INDEX_NAME, SEQ_IN_INDEX`, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Preserve first-seen order of index names while accumulating columns.
	var order []string
	byName := make(map[string]*okf.Index)
	for rows.Next() {
		var indexName, columnName string
		var nonUnique, seq int
		if err := rows.Scan(&indexName, &nonUnique, &columnName, &seq); err != nil {
			return nil, err
		}
		idx, ok := byName[indexName]
		if !ok {
			idx = &okf.Index{Name: indexName, Unique: nonUnique == 0}
			byName[indexName] = idx
			order = append(order, indexName)
		}
		idx.Columns = append(idx.Columns, columnName)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var indexes []okf.Index
	for _, name := range order {
		indexes = append(indexes, *byName[name])
	}
	return indexes, nil
}

// getConstraints reads UNIQUE and CHECK constraints for a table. UNIQUE
// constraints are grouped from KEY_COLUMN_USAGE; CHECK constraints come from the
// CHECK_CONSTRAINTS view (MySQL 8.0.16+). On older servers without that view the
// CHECK read fails silently and only UNIQUE constraints are returned.
func getConstraints(db *sql.DB, schema, table string) ([]okf.Constraint, error) {
	cons, err := getUniqueConstraints(db, schema, table)
	if err != nil {
		return nil, err
	}
	cons = append(cons, getCheckConstraints(db, schema, table)...)
	return cons, nil
}

// getUniqueConstraints groups UNIQUE constraint columns into okf.Constraint
// entries, with the column list (in ordinal order) as the definition.
func getUniqueConstraints(db *sql.DB, schema, table string) ([]okf.Constraint, error) {
	rows, err := db.Query(`
		SELECT tc.CONSTRAINT_NAME, kcu.COLUMN_NAME
		FROM information_schema.TABLE_CONSTRAINTS tc
		JOIN information_schema.KEY_COLUMN_USAGE kcu
		  ON tc.CONSTRAINT_NAME = kcu.CONSTRAINT_NAME
		 AND tc.TABLE_SCHEMA = kcu.TABLE_SCHEMA
		 AND tc.TABLE_NAME = kcu.TABLE_NAME
		WHERE tc.TABLE_SCHEMA = ? AND tc.TABLE_NAME = ? AND tc.CONSTRAINT_TYPE = 'UNIQUE'
		ORDER BY tc.CONSTRAINT_NAME, kcu.ORDINAL_POSITION`, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var order []string
	byName := make(map[string][]string)
	for rows.Next() {
		var name, col string
		if err := rows.Scan(&name, &col); err != nil {
			return nil, err
		}
		if _, ok := byName[name]; !ok {
			order = append(order, name)
		}
		byName[name] = append(byName[name], col)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var cons []okf.Constraint
	for _, name := range order {
		cons = append(cons, okf.Constraint{
			Name:       name,
			Type:       "UNIQUE",
			Definition: strings.Join(byName[name], ", "),
		})
	}
	return cons, nil
}

// getCheckConstraints reads CHECK constraints from information_schema. On MySQL
// versions lacking the CHECK_CONSTRAINTS view the query errors; we swallow that
// and return no check constraints rather than failing the whole produce.
func getCheckConstraints(db *sql.DB, schema, table string) []okf.Constraint {
	rows, err := db.Query(`
		SELECT cc.CONSTRAINT_NAME, cc.CHECK_CLAUSE
		FROM information_schema.CHECK_CONSTRAINTS cc
		JOIN information_schema.TABLE_CONSTRAINTS tc
		  ON cc.CONSTRAINT_NAME = tc.CONSTRAINT_NAME
		 AND cc.CONSTRAINT_SCHEMA = tc.TABLE_SCHEMA
		WHERE tc.TABLE_SCHEMA = ? AND tc.TABLE_NAME = ?
		ORDER BY cc.CONSTRAINT_NAME`, schema, table)
	if err != nil {
		return nil // older MySQL without CHECK_CONSTRAINTS view
	}
	defer rows.Close()

	var cons []okf.Constraint
	for rows.Next() {
		var name, clause string
		if err := rows.Scan(&name, &clause); err != nil {
			return cons
		}
		cons = append(cons, okf.Constraint{Name: name, Type: "CHECK", Definition: clause})
	}
	return cons
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
func getTableStats(db *sql.DB, table string, cols []ColumnSpec) (okf.TableStats, error) {
	var s okf.TableStats
	var cnt int64
	if err := db.QueryRow("SELECT COUNT(*) FROM `" + table + "`").Scan(&cnt); err != nil {
		return s, err
	}
	s.RowCount = cnt
	s.HasRowCount = true

	if col := detectFreshnessColumn(cols); col != "" {
		var mn, mx sql.NullString
		q := fmt.Sprintf("SELECT CAST(MIN(`%[1]s`) AS CHAR), CAST(MAX(`%[1]s`) AS CHAR) FROM `%[2]s`", col, table)
		if err := db.QueryRow(q).Scan(&mn, &mx); err == nil {
			s.FreshnessColumn = col
			s.Earliest = mn.String
			s.Latest = mx.String
		}
	}
	return s, nil
}

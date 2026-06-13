package main

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/savikne/okf-skills-registry/okf-go"
)

// quoteIdent double-quotes a PostgreSQL identifier, escaping embedded quotes.
func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// profileTable computes per-column statistics for a PostgreSQL table within a schema.
func profileTable(db *sql.DB, schema, table string, cols []ColumnSpec) ([]okf.ColumnProfile, error) {
	rel := quoteIdent(schema) + "." + quoteIdent(table)
	var profiles []okf.ColumnProfile
	for _, c := range cols {
		col := quoteIdent(c.Name)
		q := fmt.Sprintf(
			"SELECT COUNT(%[1]s), COUNT(*) - COUNT(%[1]s), COUNT(DISTINCT %[1]s), "+
				"MIN(%[1]s)::text, MAX(%[1]s)::text FROM %[2]s",
			col, rel)
		var nonNull, null, distinct int64
		var min, max sql.NullString
		if err := db.QueryRow(q).Scan(&nonNull, &null, &distinct, &min, &max); err != nil {
			return nil, fmt.Errorf("profile column %s.%s: %w", table, c.Name, err)
		}
		profiles = append(profiles, okf.ColumnProfile{
			Column: c.Name, NonNull: nonNull, Null: null, Distinct: distinct,
			Min: min.String, Max: max.String,
		})
	}
	return profiles, nil
}

// sampleTable returns up to limit rows from a PostgreSQL table as string headers + cells.
func sampleTable(db *sql.DB, schema, table string, limit int) ([]string, [][]string, error) {
	rel := quoteIdent(schema) + "." + quoteIdent(table)
	rows, err := db.Query(fmt.Sprintf("SELECT * FROM %s LIMIT %d", rel, limit))
	if err != nil {
		return nil, nil, fmt.Errorf("sample %s: %w", table, err)
	}
	defer rows.Close()

	headers, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}

	var out [][]string
	for rows.Next() {
		cells := make([]sql.NullString, len(headers))
		ptrs := make([]any, len(headers))
		for i := range cells {
			ptrs[i] = &cells[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, nil, err
		}
		row := make([]string, len(headers))
		for i, c := range cells {
			if c.Valid {
				row[i] = c.String
			} else {
				row[i] = "NULL"
			}
		}
		out = append(out, row)
	}
	return headers, out, rows.Err()
}

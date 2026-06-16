package main

import (
	"database/sql"
	"fmt"

	"github.com/xSAVIKx/okf-skills/okf-go"
)

// profileTable computes per-column statistics for a MySQL table.
func profileTable(db *sql.DB, table string, cols []ColumnSpec) ([]okf.ColumnProfile, error) {
	var profiles []okf.ColumnProfile
	for _, c := range cols {
		q := fmt.Sprintf(
			"SELECT COUNT(`%[1]s`), COUNT(*) - COUNT(`%[1]s`), COUNT(DISTINCT `%[1]s`), "+
				"CAST(MIN(`%[1]s`) AS CHAR), CAST(MAX(`%[1]s`) AS CHAR) FROM `%[2]s`",
			c.Name, table)
		var nonNull, null, distinct int64
		var min, max sql.NullString
		if err := db.QueryRow(q).Scan(&nonNull, &null, &distinct, &min, &max); err != nil {
			return nil, fmt.Errorf("profile column %s.%s: %w", table, c.Name, err)
		}
		// Pull up to LowCardinalityN+1 distinct values (LIMIT-bounded, no full scan)
		// to drive semantic-type detection and the literal value set.
		distinctVals, err := distinctValues(db, table, c.Name, okf.LowCardinalityN+1)
		if err != nil {
			return nil, fmt.Errorf("distinct values %s.%s: %w", table, c.Name, err)
		}
		semantic, values := okf.ClassifyColumn(c.Name, distinctVals, distinct)
		profiles = append(profiles, okf.ColumnProfile{
			Column: c.Name, NonNull: nonNull, Null: null, Distinct: distinct,
			Min: min.String, Max: max.String,
			Semantic: semantic, Values: values,
		})
	}
	return profiles, nil
}

// distinctValues returns up to limit distinct non-null values of a column as text.
// The LIMIT keeps this a cheap, bounded read on high-cardinality columns; the
// ORDER BY makes WHICH values the LIMIT returns deterministic across runs, so the
// derived semantic tag (and thus the concept body) is byte-stable.
func distinctValues(db *sql.DB, table, col string, limit int) ([]string, error) {
	q := fmt.Sprintf("SELECT DISTINCT CAST(`%[1]s` AS CHAR) FROM `%[2]s` WHERE `%[1]s` IS NOT NULL ORDER BY 1 LIMIT %[3]d", col, table, limit)
	rows, err := db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v sql.NullString
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		if v.Valid {
			out = append(out, v.String)
		}
	}
	return out, rows.Err()
}

// sampleTable returns up to limit rows from a MySQL table as string headers + cells.
func sampleTable(db *sql.DB, table string, limit int) ([]string, [][]string, error) {
	rows, err := db.Query(fmt.Sprintf("SELECT * FROM `%s` LIMIT %d", table, limit))
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

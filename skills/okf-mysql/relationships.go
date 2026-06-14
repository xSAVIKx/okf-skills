package main

import (
	"database/sql"
	"fmt"

	okf "github.com/xSAVIKx/okf-skills/okf-go"
)

// foreignKey is the subset of an information_schema.KEY_COLUMN_USAGE FK row we turn
// into an edge: a column in the current table that references a column in another table.
type foreignKey struct {
	FromColumn string // column in the current table
	ToTable    string // referenced table
	ToColumn   string // referenced column
}

// getForeignKeys reads the declared foreign keys of the given table from
// information_schema.KEY_COLUMN_USAGE. Purely a catalog read — no LLM, no data scan.
func getForeignKeys(db *sql.DB, schema, table string) ([]foreignKey, error) {
	rows, err := db.Query(`
		SELECT COLUMN_NAME, REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME
		FROM information_schema.KEY_COLUMN_USAGE
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND REFERENCED_TABLE_NAME IS NOT NULL
		ORDER BY ORDINAL_POSITION`, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fks []foreignKey
	for rows.Next() {
		var from, refTable, refColumn sql.NullString
		if err := rows.Scan(&from, &refTable, &refColumn); err != nil {
			return nil, err
		}
		fks = append(fks, foreignKey{
			FromColumn: from.String,
			ToTable:    refTable.String,
			ToColumn:   refColumn.String,
		})
	}
	return fks, rows.Err()
}

// foreignKeyRelationships maps a table's foreign keys to okf.Relationship edges,
// each targeting the referenced table's concept doc. Pure and deterministic so it
// is unit-testable without a database; rendering order is handled downstream by
// okf.RenderRelationshipsSection.
func foreignKeyRelationships(fks []foreignKey) []okf.Relationship {
	var rels []okf.Relationship
	for _, fk := range fks {
		if fk.ToTable == "" {
			continue
		}
		rels = append(rels, okf.Relationship{
			Label:  fmt.Sprintf("FK on %s", fk.FromColumn),
			Target: fmt.Sprintf("/tables/%s.md", fk.ToTable),
			Text:   fk.ToTable,
		})
	}
	return rels
}

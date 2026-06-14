package main

import (
	"database/sql"
	"fmt"

	okf "github.com/xSAVIKx/okf-skills/okf-go"
)

// foreignKey is the subset of a PRAGMA foreign_key_list row we turn into an edge:
// a column in the current table that references a column in another table.
type foreignKey struct {
	FromColumn string // column in the current table
	ToTable    string // referenced table
	ToColumn   string // referenced column ("" when the FK targets the table's PK)
}

// getForeignKeys reads PRAGMA foreign_key_list(<table>) and returns the declared
// foreign keys of the table. Purely a catalog read — no LLM, no data scan.
func getForeignKeys(db *sql.DB, table string) ([]foreignKey, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA foreign_key_list(`%s`)", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fks []foreignKey
	for rows.Next() {
		// Columns: id, seq, table, from, to, on_update, on_delete, match.
		var id, seq int
		var refTable, from, to, onUpdate, onDelete, match sql.NullString
		if err := rows.Scan(&id, &seq, &refTable, &from, &to, &onUpdate, &onDelete, &match); err != nil {
			return nil, err
		}
		fks = append(fks, foreignKey{
			FromColumn: from.String,
			ToTable:    refTable.String,
			ToColumn:   to.String,
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

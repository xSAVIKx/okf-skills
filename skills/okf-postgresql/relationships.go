package main

import (
	"database/sql"
	"fmt"

	okf "github.com/xSAVIKx/okf-skills/okf-go"
)

// foreignKey is the subset of an information_schema foreign-key row we turn into
// an edge: a column in the current table that references a column in another table.
type foreignKey struct {
	FromColumn string // column in the current table
	ToTable    string // referenced table
	ToColumn   string // referenced column
}

// getForeignKeys reads the declared foreign keys of a table from the standard
// information_schema catalog views. Purely a catalog read — no LLM, no data scan.
func getForeignKeys(db *sql.DB, schema, table string) ([]foreignKey, error) {
	rows, err := db.Query(`
		SELECT kcu.column_name, ccu.table_name AS ref_table, ccu.column_name AS ref_column
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		  ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage ccu
		  ON ccu.constraint_name = tc.constraint_name AND ccu.table_schema = tc.table_schema
		WHERE tc.constraint_type = 'FOREIGN KEY' AND tc.table_schema = $1 AND tc.table_name = $2
		ORDER BY kcu.ordinal_position`, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fks []foreignKey
	for rows.Next() {
		var fromCol, refTable, refColumn sql.NullString
		if err := rows.Scan(&fromCol, &refTable, &refColumn); err != nil {
			return nil, err
		}
		fks = append(fks, foreignKey{
			FromColumn: fromCol.String,
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

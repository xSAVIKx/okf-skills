package main

import (
	"context"
	"fmt"
	"log"

	"cloud.google.com/go/bigquery"
	"github.com/xSAVIKx/okf-skills/okf-go"
	"google.golang.org/api/iterator"
)

// foreignKey is the subset of a declared foreign-key constraint we turn into an
// edge: a column in the current table that references a column in another table.
// BigQuery foreign keys are informational/unenforced but are recorded in
// INFORMATION_SCHEMA, which is what we read here.
type foreignKey struct {
	FromColumn string // column in the current table
	ToTable    string // referenced table
	ToColumn   string // referenced column
}

// getForeignKeys queries the dataset's INFORMATION_SCHEMA for the declared
// foreign-key constraints of a single table. Purely a catalog read — no LLM, no
// data scan. Older datasets (or those without the constraint views) may fail the
// query; in that case we log a warning and return an empty slice + nil error so
// produce never hard-fails on a dataset lacking FK metadata.
func getForeignKeys(ctx context.Context, client *bigquery.Client, projectID, datasetID, table string) ([]foreignKey, error) {
	q := client.Query(fmt.Sprintf(
		"SELECT kcu.column_name AS from_column, ccu.table_name AS to_table, ccu.column_name AS to_column\n"+
			"FROM `%[1]s.%[2]s`.INFORMATION_SCHEMA.TABLE_CONSTRAINTS tc\n"+
			"JOIN `%[1]s.%[2]s`.INFORMATION_SCHEMA.KEY_COLUMN_USAGE kcu\n"+
			"  ON tc.constraint_name = kcu.constraint_name\n"+
			"JOIN `%[1]s.%[2]s`.INFORMATION_SCHEMA.CONSTRAINT_COLUMN_USAGE ccu\n"+
			"  ON tc.constraint_name = ccu.constraint_name\n"+
			"WHERE tc.constraint_type = 'FOREIGN KEY' AND tc.table_name = @table\n"+
			"ORDER BY kcu.ordinal_position", projectID, datasetID))
	q.Parameters = []bigquery.QueryParameter{{Name: "table", Value: table}}

	it, err := q.Read(ctx)
	if err != nil {
		log.Printf("Warning: foreign-key metadata unavailable for table %s: %v", table, err)
		return nil, nil
	}

	var fks []foreignKey
	for {
		var row struct {
			FromColumn string `bigquery:"from_column"`
			ToTable    string `bigquery:"to_table"`
			ToColumn   string `bigquery:"to_column"`
		}
		err := it.Next(&row)
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Printf("Warning: failed reading foreign-key row for table %s: %v", table, err)
			return nil, nil
		}
		fks = append(fks, foreignKey{
			FromColumn: row.FromColumn,
			ToTable:    row.ToTable,
			ToColumn:   row.ToColumn,
		})
	}
	return fks, nil
}

// foreignKeyRelationships maps a table's foreign keys to okf.Relationship edges,
// each targeting the referenced table's concept doc. Pure and deterministic so it
// is unit-testable without a database; rendering order is handled downstream by
// okf.AppendRelationshipsSection.
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

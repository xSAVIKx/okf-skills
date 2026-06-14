package main

import (
	"context"
	"fmt"
	"log"
	"regexp"

	"cloud.google.com/go/bigquery"
	"github.com/xSAVIKx/okf-skills/okf-go"
	"google.golang.org/api/iterator"
)

// getConstraints queries the dataset's INFORMATION_SCHEMA for UNIQUE and PRIMARY
// KEY informational constraints of a single table and maps them to okf.Constraint.
// BigQuery has no CHECK constraints, so none are produced for that type. Purely a
// catalog read — no LLM, no data scan. Older datasets (or those without the
// constraint views) may fail the query; in that case we log a warning and return
// an empty slice + nil error so produce never hard-fails on a dataset lacking
// constraint metadata (tolerant degradation to empty).
func getConstraints(ctx context.Context, client *bigquery.Client, projectID, datasetID, table string) ([]okf.Constraint, error) {
	q := client.Query(fmt.Sprintf(
		"SELECT tc.constraint_name AS name, tc.constraint_type AS ctype,\n"+
			"  STRING_AGG(kcu.column_name, ', ' ORDER BY kcu.ordinal_position) AS cols\n"+
			"FROM `%[1]s.%[2]s`.INFORMATION_SCHEMA.TABLE_CONSTRAINTS tc\n"+
			"JOIN `%[1]s.%[2]s`.INFORMATION_SCHEMA.KEY_COLUMN_USAGE kcu\n"+
			"  ON tc.constraint_name = kcu.constraint_name\n"+
			"WHERE tc.table_name = @table\n"+
			"  AND tc.constraint_type IN ('UNIQUE', 'PRIMARY KEY')\n"+
			"GROUP BY tc.constraint_name, tc.constraint_type", projectID, datasetID))
	q.Parameters = []bigquery.QueryParameter{{Name: "table", Value: table}}

	it, err := q.Read(ctx)
	if err != nil {
		log.Printf("Warning: constraint metadata unavailable for table %s: %v", table, err)
		return nil, nil
	}

	var cons []okf.Constraint
	for {
		var row struct {
			Name  string `bigquery:"name"`
			CType string `bigquery:"ctype"`
			Cols  string `bigquery:"cols"`
		}
		err := it.Next(&row)
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Printf("Warning: failed reading constraint row for table %s: %v", table, err)
			return nil, nil
		}
		cons = append(cons, okf.Constraint{
			Name:       row.Name,
			Type:       row.CType,
			Definition: row.Cols,
		})
	}
	return cons, nil
}

// BigQuery has no traditional secondary indexes, so the connector emits no
// Indexes section. There is intentionally no getIndexes here.

var freshnessTypeHint = regexp.MustCompile(`(?i)DATE|TIME|TIMESTAMP`)
var freshnessNameHint = regexp.MustCompile(`(?i)(^|_)(date|time|timestamp)|_at$|_on$`)

// detectFreshnessColumn picks a timestamp-like column to drive freshness stats,
// preferring a date/time *type* (TIMESTAMP/DATE/DATETIME) and falling back to a
// date/time-ish *name*. Operates over the connector's already-fetched schema so
// it is pure and unit-testable. Mirrors the sqlite connector's detection helper.
func detectFreshnessColumn(fields bigquery.Schema) string {
	for _, f := range fields {
		if freshnessTypeHint.MatchString(string(f.Type)) {
			return f.Name
		}
	}
	for _, f := range fields {
		if freshnessNameHint.MatchString(f.Name) {
			return f.Name
		}
	}
	return ""
}

// getTableStats builds table-level statistics. The row count comes for free from
// the already-fetched table metadata (md.NumRows) — no query, no data scan. If a
// timestamp-like column is detected, a single tolerant MIN/MAX query computes the
// freshness window. Any query error degrades to an empty freshness window rather
// than hard-failing. Gated by the connector's --stats flag.
func getTableStats(ctx context.Context, client *bigquery.Client, projectID, datasetID, table string, numRows uint64, fields bigquery.Schema) (okf.TableStats, error) {
	s := okf.TableStats{
		RowCount:    int64(numRows),
		HasRowCount: true,
	}

	col := detectFreshnessColumn(fields)
	if col == "" {
		return s, nil
	}

	ref := fmt.Sprintf("`%s.%s.%s`", projectID, datasetID, table)
	q := client.Query(fmt.Sprintf(
		"SELECT CAST(MIN(`%[1]s`) AS STRING) AS min_v, CAST(MAX(`%[1]s`) AS STRING) AS max_v FROM %[2]s",
		col, ref))
	it, err := q.Read(ctx)
	if err != nil {
		log.Printf("Warning: freshness query unavailable for table %s: %v", table, err)
		return s, nil
	}
	var row struct {
		MinV string `bigquery:"min_v"`
		MaxV string `bigquery:"max_v"`
	}
	if err := it.Next(&row); err != nil && err != iterator.Done {
		log.Printf("Warning: failed reading freshness row for table %s: %v", table, err)
		return s, nil
	}
	s.FreshnessColumn = col
	s.Earliest = row.MinV
	s.Latest = row.MaxV
	return s, nil
}

package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"cloud.google.com/go/bigquery"
	"github.com/xSAVIKx/okf-skills/okf-go"
	"google.golang.org/api/iterator"
)

// profileTable computes per-column statistics for a BigQuery table by running
// one aggregate query per column (one BigQuery job each).
func profileTable(ctx context.Context, client *bigquery.Client, projectID, datasetID, table string, fields bigquery.Schema) ([]okf.ColumnProfile, error) {
	ref := fmt.Sprintf("`%s.%s.%s`", projectID, datasetID, table)
	var profiles []okf.ColumnProfile
	for _, f := range fields {
		col := "`" + f.Name + "`"
		q := client.Query(fmt.Sprintf(
			"SELECT COUNT(%[1]s) AS non_null, COUNT(*) - COUNT(%[1]s) AS nulls, "+
				"COUNT(DISTINCT %[1]s) AS distinct_ct, CAST(MIN(%[1]s) AS STRING) AS min_v, "+
				"CAST(MAX(%[1]s) AS STRING) AS max_v FROM %[2]s", col, ref))
		it, err := q.Read(ctx)
		if err != nil {
			return nil, fmt.Errorf("profile column %s.%s: %w", table, f.Name, err)
		}
		var row struct {
			NonNull    int64  `bigquery:"non_null"`
			Nulls      int64  `bigquery:"nulls"`
			DistinctCt int64  `bigquery:"distinct_ct"`
			MinV       string `bigquery:"min_v"`
			MaxV       string `bigquery:"max_v"`
		}
		if err := it.Next(&row); err != nil && err != iterator.Done {
			return nil, fmt.Errorf("read profile %s.%s: %w", table, f.Name, err)
		}
		// Pull up to LowCardinalityN+1 distinct values (LIMIT-bounded, no full scan)
		// to drive semantic-type detection and the literal value set. A failure here
		// is non-fatal: warn and proceed without semantic/values rather than aborting
		// the whole produce.
		distinctVals, err := distinctValues(ctx, client, ref, f.Name, okf.LowCardinalityN+1)
		if err != nil {
			log.Printf("Warning: distinct values unavailable for column %s.%s: %v", table, f.Name, err)
			distinctVals = nil
		}
		semantic, values := okf.ClassifyColumn(f.Name, distinctVals, row.DistinctCt)
		profiles = append(profiles, okf.ColumnProfile{
			Column: f.Name, NonNull: row.NonNull, Null: row.Nulls, Distinct: row.DistinctCt,
			Min: row.MinV, Max: row.MaxV, Semantic: semantic, Values: values,
		})
	}
	return profiles, nil
}

// distinctValues returns up to limit distinct non-null values of a column as text.
// The LIMIT bounds the scan on high-cardinality columns; the ORDER BY makes WHICH
// values the LIMIT returns deterministic across runs, so the derived semantic tag
// (and thus the concept body) is byte-stable.
func distinctValues(ctx context.Context, client *bigquery.Client, ref, col string, limit int) ([]string, error) {
	q := client.Query(fmt.Sprintf(
		"SELECT DISTINCT CAST(`%[1]s` AS STRING) AS v FROM %[2]s WHERE `%[1]s` IS NOT NULL ORDER BY 1 LIMIT %[3]d",
		col, ref, limit))
	it, err := q.Read(ctx)
	if err != nil {
		return nil, err
	}
	var out []string
	for {
		var row struct {
			V bigquery.NullString `bigquery:"v"`
		}
		err := it.Next(&row)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		if row.V.Valid {
			out = append(out, row.V.StringVal)
		}
	}
	return out, nil
}

// sampleTable returns up to limit rows from a BigQuery table as string headers + cells.
func sampleTable(ctx context.Context, client *bigquery.Client, projectID, datasetID, table string, limit int, fields bigquery.Schema) ([]string, [][]string, error) {
	ref := fmt.Sprintf("`%s.%s.%s`", projectID, datasetID, table)
	q := client.Query(fmt.Sprintf("SELECT * FROM %s LIMIT %d", ref, limit))
	it, err := q.Read(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("sample %s: %w", table, err)
	}

	headers := make([]string, len(fields))
	for i, f := range fields {
		headers[i] = f.Name
	}

	var out [][]string
	for {
		var vals []bigquery.Value
		err := it.Next(&vals)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, nil, err
		}
		row := make([]string, len(vals))
		for i, v := range vals {
			if v == nil {
				row[i] = "NULL"
			} else {
				row[i] = strings.TrimSpace(fmt.Sprintf("%v", v))
			}
		}
		out = append(out, row)
	}
	return headers, out, nil
}

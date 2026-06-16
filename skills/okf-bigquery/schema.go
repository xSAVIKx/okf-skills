package main

import "github.com/xSAVIKx/okf-skills/okf-go"

// buildSchema returns the machine-readable self-description of the okf-bigquery skill.
func buildSchema() okf.SkillSchema {
	return okf.SkillSchema{
		Name:        "okf-bigquery",
		Description: "Produce OKF bundles from a BigQuery dataset and sync descriptions back.",
		Commands: []okf.CommandSchema{
			{
				Name:        "produce",
				Description: "Create an OKF bundle from a BigQuery dataset.",
				Flags: []okf.FlagSchema{
					{Name: "project", Type: "string", Description: "GCP Project ID.", Required: true},
					{Name: "dataset", Type: "string", Description: "BigQuery Dataset ID.", Required: true},
					{Name: "out", Type: "string", Description: "Output OKF bundle directory.", Required: true},
					{Name: "tables", Type: "string", Description: "Comma-separated tables to extract (optional)."},
					{Name: "sample", Type: "int", Description: "Sample rows to embed per table (0 = none).", Default: "0"},
					{Name: "profile", Type: "bool", Description: "Embed a per-column Data Profile section.", Default: "false"},
					{Name: "relationships", Type: "bool", Description: "Extract foreign-key constraints into a Relationships section.", Default: "false"},
					{Name: "stats", Type: "bool", Description: "Compute row-count and freshness statistics (Stats section).", Default: "false"},
				},
			},
			{
				Name:        "ingest",
				Description: "Sync OKF bundle descriptions back to BigQuery.",
				Flags: []okf.FlagSchema{
					{Name: "project", Type: "string", Description: "GCP Project ID.", Required: true},
					{Name: "dataset", Type: "string", Description: "BigQuery Dataset ID.", Required: true},
					{Name: "bundle", Type: "string", Description: "Path to existing OKF bundle directory.", Required: true},
					{Name: "sync", Type: "bool", Description: "Write descriptions back to BigQuery.", Default: "false"},
				},
			},
			{
				Name:        "schema",
				Description: "Print this skill's machine-readable JSON self-description.",
				Flags:       []okf.FlagSchema{},
			},
		},
	}
}

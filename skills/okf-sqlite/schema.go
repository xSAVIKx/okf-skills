package main

import "github.com/xSAVIKx/okf-skills/okf-go"

// buildSchema returns the machine-readable self-description of the okf-sqlite skill.
func buildSchema() okf.SkillSchema {
	return okf.SkillSchema{
		Name:        "okf-sqlite",
		Description: "Produce OKF bundles from a SQLite database and ingest/sync schema back.",
		Commands: []okf.CommandSchema{
			{
				Name:        "produce",
				Description: "Create an OKF bundle from a SQLite database file.",
				Flags: []okf.FlagSchema{
					{Name: "db", Type: "string", Description: "Path to SQLite database file.", Required: true},
					{Name: "out", Type: "string", Description: "Output OKF bundle directory.", Required: true},
					{Name: "tables", Type: "string", Description: "Comma-separated tables to extract (optional)."},
					{Name: "sample", Type: "int", Description: "Sample rows to embed per table (0 = none).", Default: "0"},
					{Name: "profile", Type: "bool", Description: "Embed a per-column Data Profile section.", Default: "false"},
					{Name: "relationships", Type: "bool", Description: "Extract foreign-key constraints into a Relationships section.", Default: "false"},
				},
			},
			{
				Name:        "ingest",
				Description: "Verify or sync an OKF bundle schema into a SQLite database.",
				Flags: []okf.FlagSchema{
					{Name: "db", Type: "string", Description: "Path to SQLite database file.", Required: true},
					{Name: "bundle", Type: "string", Description: "Path to existing OKF bundle directory.", Required: true},
					{Name: "sync", Type: "bool", Description: "Create missing tables/columns.", Default: "false"},
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

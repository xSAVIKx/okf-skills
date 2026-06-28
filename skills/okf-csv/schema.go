package main

import "github.com/xSAVIKx/okf-skills/okf-go"

// buildSchema returns the machine-readable self-description of the okf-csv skill.
func buildSchema() okf.SkillSchema {
	return okf.SkillSchema{
		Name:        "okf-csv",
		Description: "Produce and ingest OKF bundles from a directory of CSV files, inferring column types and profiles.",
		Commands: []okf.CommandSchema{
			{
				Name:        "produce",
				Description: "Create an OKF bundle from a directory of CSV files.",
				Flags: []okf.FlagSchema{
					{Name: "dir", Type: "string", Description: "Directory of CSV files to document.", Required: true},
					{Name: "out", Type: "string", Description: "Output OKF bundle directory.", Required: true},
					{Name: "sample", Type: "int", Description: "Sample rows to embed per file (0 = none).", Default: "0"},
					{Name: "profile", Type: "bool", Description: "Embed a per-column Data Profile section.", Default: "false"},
				},
			},
			{
				Name:        "ingest",
				Description: "Sync OKF bundle descriptions back to the directory's .okf-metadata.yaml.",
				Flags: []okf.FlagSchema{
					{Name: "dir", Type: "string", Description: "Local directory.", Required: true},
					{Name: "bundle", Type: "string", Description: "Path to existing OKF bundle directory.", Required: true},
					{Name: "sync", Type: "bool", Description: "Write descriptions back to .okf-metadata.yaml.", Default: "false"},
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

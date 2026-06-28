package main

import "github.com/xSAVIKx/okf-skills/okf-go"

// buildSchema returns the machine-readable self-description of the okf-mongodb skill.
func buildSchema() okf.SkillSchema {
	return okf.SkillSchema{
		Name:        "okf-mongodb",
		Description: "Produce and ingest OKF bundles from a MongoDB database by sampling documents to infer collection schemas.",
		Commands: []okf.CommandSchema{
			{
				Name:        "produce",
				Description: "Create an OKF bundle from a MongoDB database by sampling documents.",
				Flags: []okf.FlagSchema{
					{Name: "uri", Type: "string", Description: "MongoDB connection URI.", Required: true, Env: "MONGODB_URI"},
					{Name: "db", Type: "string", Description: "Database name.", Required: true},
					{Name: "out", Type: "string", Description: "Output OKF bundle directory.", Required: true},
					{Name: "collections", Type: "string", Description: "Comma-separated collections to extract (optional)."},
					{Name: "sample", Type: "int", Description: "Documents to sample per collection for schema inference.", Default: "100"},
				},
			},
			{
				Name:        "ingest",
				Description: "Verify a bundle against the database; --sync creates missing collections.",
				Flags: []okf.FlagSchema{
					{Name: "uri", Type: "string", Description: "MongoDB connection URI.", Required: true, Env: "MONGODB_URI"},
					{Name: "db", Type: "string", Description: "Database name.", Required: true},
					{Name: "bundle", Type: "string", Description: "Path to existing OKF bundle directory.", Required: true},
					{Name: "sync", Type: "bool", Description: "Create missing collections (structure only).", Default: "false"},
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

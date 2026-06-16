package main

import (
	"os"

	"github.com/xSAVIKx/okf-skills/okf-go"
)

// resolvePassword returns the explicit flag value if set, otherwise the
// PGPASSWORD environment variable. This keeps secrets out of argv.
func resolvePassword(flagVal string) string {
	if flagVal != "" {
		return flagVal
	}
	return os.Getenv("PGPASSWORD")
}

// buildSchema returns the machine-readable self-description of the okf-postgresql skill.
func buildSchema() okf.SkillSchema {
	conn := []okf.FlagSchema{
		{Name: "host", Type: "string", Description: "PostgreSQL host.", Default: "localhost"},
		{Name: "port", Type: "int", Description: "PostgreSQL port.", Default: "5432"},
		{Name: "user", Type: "string", Description: "PostgreSQL user.", Default: "postgres"},
		{Name: "password", Type: "string", Description: "PostgreSQL password (or set PGPASSWORD).", Required: true, Env: "PGPASSWORD"},
		{Name: "db", Type: "string", Description: "PostgreSQL database name.", Required: true},
		{Name: "schema", Type: "string", Description: "PostgreSQL schema name.", Default: "public"},
	}
	produce := append(append([]okf.FlagSchema{}, conn...),
		okf.FlagSchema{Name: "out", Type: "string", Description: "Output OKF bundle directory.", Required: true},
		okf.FlagSchema{Name: "tables", Type: "string", Description: "Comma-separated tables to extract (optional)."},
		okf.FlagSchema{Name: "sample", Type: "int", Description: "Sample rows to embed per table (0 = none).", Default: "0"},
		okf.FlagSchema{Name: "profile", Type: "bool", Description: "Embed a per-column Data Profile section.", Default: "false"},
		okf.FlagSchema{Name: "relationships", Type: "bool", Description: "Extract foreign-key constraints into a Relationships section.", Default: "false"},
	)
	ingest := append(append([]okf.FlagSchema{}, conn...),
		okf.FlagSchema{Name: "bundle", Type: "string", Description: "Path to existing OKF bundle directory.", Required: true},
		okf.FlagSchema{Name: "sync", Type: "bool", Description: "Write comment changes back to PostgreSQL.", Default: "false"},
	)
	return okf.SkillSchema{
		Name:        "okf-postgresql",
		Description: "Produce OKF bundles from a PostgreSQL database and sync comments back.",
		Commands: []okf.CommandSchema{
			{Name: "produce", Description: "Create an OKF bundle from a PostgreSQL database.", Flags: produce},
			{Name: "ingest", Description: "Sync OKF bundle comments back into a PostgreSQL database.", Flags: ingest},
			{Name: "schema", Description: "Print this skill's machine-readable JSON self-description.", Flags: []okf.FlagSchema{}},
		},
	}
}

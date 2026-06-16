package main

import (
	"os"

	"github.com/xSAVIKx/okf-skills/okf-go"
)

// resolvePassword returns the explicit flag value if set, otherwise the
// MYSQL_PASSWORD environment variable. This keeps secrets out of argv.
func resolvePassword(flagVal string) string {
	if flagVal != "" {
		return flagVal
	}
	return os.Getenv("MYSQL_PASSWORD")
}

// buildSchema returns the machine-readable self-description of the okf-mysql skill.
func buildSchema() okf.SkillSchema {
	conn := []okf.FlagSchema{
		{Name: "host", Type: "string", Description: "MySQL host.", Default: "localhost"},
		{Name: "port", Type: "int", Description: "MySQL port.", Default: "3306"},
		{Name: "user", Type: "string", Description: "MySQL user.", Required: true},
		{Name: "password", Type: "string", Description: "MySQL password (or set MYSQL_PASSWORD).", Required: true, Env: "MYSQL_PASSWORD"},
		{Name: "db", Type: "string", Description: "MySQL database/schema name.", Required: true},
	}
	produce := append(append([]okf.FlagSchema{}, conn...),
		okf.FlagSchema{Name: "out", Type: "string", Description: "Output OKF bundle directory.", Required: true},
		okf.FlagSchema{Name: "tables", Type: "string", Description: "Comma-separated tables to extract (optional)."},
		okf.FlagSchema{Name: "sample", Type: "int", Description: "Sample rows to embed per table (0 = none).", Default: "0"},
		okf.FlagSchema{Name: "profile", Type: "bool", Description: "Embed a per-column Data Profile section.", Default: "false"},
		okf.FlagSchema{Name: "relationships", Type: "bool", Description: "Extract foreign-key relationships and embed a Relationships section.", Default: "false"},
	)
	ingest := append(append([]okf.FlagSchema{}, conn...),
		okf.FlagSchema{Name: "bundle", Type: "string", Description: "Path to existing OKF bundle directory.", Required: true},
		okf.FlagSchema{Name: "sync", Type: "bool", Description: "Write comment changes back to MySQL.", Default: "false"},
	)
	return okf.SkillSchema{
		Name:        "okf-mysql",
		Description: "Produce OKF bundles from a MySQL database and sync comments back.",
		Commands: []okf.CommandSchema{
			{Name: "produce", Description: "Create an OKF bundle from a MySQL database.", Flags: produce},
			{Name: "ingest", Description: "Sync OKF bundle comments back into a MySQL database.", Flags: ingest},
			{Name: "schema", Description: "Print this skill's machine-readable JSON self-description.", Flags: []okf.FlagSchema{}},
		},
	}
}

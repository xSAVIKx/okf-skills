package main

import "github.com/xSAVIKx/okf-skills/okf-go"

// buildSchema returns the machine-readable self-description of the okf-git skill.
func buildSchema() okf.SkillSchema {
	return okf.SkillSchema{
		Name:        "okf-git",
		Description: "Produce OKF bundles from a local Git repository and sync descriptions back.",
		Commands: []okf.CommandSchema{
			{
				Name:        "produce",
				Description: "Create an OKF bundle from a local Git repository.",
				Flags: []okf.FlagSchema{
					{Name: "repo", Type: "string", Description: "Git repository path.", Required: true},
					{Name: "out", Type: "string", Description: "Output OKF bundle directory.", Required: true},
				},
			},
			{
				Name:        "ingest",
				Description: "Sync OKF bundle descriptions back to the repo's .okf-metadata.yaml.",
				Flags: []okf.FlagSchema{
					{Name: "repo", Type: "string", Description: "Git repository path.", Required: true},
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

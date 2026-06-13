package main

import "github.com/savikne/okf-skills-registry/okf-go"

// buildSchema returns the machine-readable self-description of the okf-fs skill.
func buildSchema() okf.SkillSchema {
	return okf.SkillSchema{
		Name:        "okf-fs",
		Description: "Produce OKF bundles from a local directory tree and sync descriptions back.",
		Commands: []okf.CommandSchema{
			{
				Name:        "produce",
				Description: "Create an OKF bundle from a local directory.",
				Flags: []okf.FlagSchema{
					{Name: "dir", Type: "string", Description: "Local directory to document.", Required: true},
					{Name: "out", Type: "string", Description: "Output OKF bundle directory.", Required: true},
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

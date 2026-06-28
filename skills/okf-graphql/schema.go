package main

import "github.com/xSAVIKx/okf-skills/okf-go"

// buildSchema returns the machine-readable self-description of the okf-graphql skill.
func buildSchema() okf.SkillSchema {
	return okf.SkillSchema{
		Name:        "okf-graphql",
		Description: "Produce and ingest OKF bundles from a GraphQL SDL: types, queries, and mutations with relationship edges.",
		Commands: []okf.CommandSchema{
			{
				Name:        "produce",
				Description: "Create an OKF bundle from a GraphQL SDL document.",
				Flags: []okf.FlagSchema{
					{Name: "schema", Type: "string", Description: "Path to the GraphQL SDL file.", Required: true},
					{Name: "out", Type: "string", Description: "Output OKF bundle directory.", Required: true},
				},
			},
			{
				Name:        "ingest",
				Description: "Verify a bundle against the schema; --sync writes descriptions to .okf-metadata.yaml.",
				Flags: []okf.FlagSchema{
					{Name: "schema", Type: "string", Description: "Path to the GraphQL SDL file.", Required: true},
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

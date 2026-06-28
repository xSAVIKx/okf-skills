package main

import "github.com/xSAVIKx/okf-skills/okf-go"

// buildSchema returns the machine-readable self-description of the okf-openapi skill.
func buildSchema() okf.SkillSchema {
	return okf.SkillSchema{
		Name:        "okf-openapi",
		Description: "Produce and ingest OKF bundles from an OpenAPI/Swagger spec: one concept per endpoint and schema.",
		Commands: []okf.CommandSchema{
			{
				Name:        "produce",
				Description: "Create an OKF bundle from an OpenAPI/Swagger spec.",
				Flags: []okf.FlagSchema{
					{Name: "spec", Type: "string", Description: "Path to the OpenAPI/Swagger spec file (JSON or YAML).", Required: true},
					{Name: "out", Type: "string", Description: "Output OKF bundle directory.", Required: true},
				},
			},
			{
				Name:        "ingest",
				Description: "Verify a bundle against the spec; --sync writes descriptions to .okf-metadata.yaml.",
				Flags: []okf.FlagSchema{
					{Name: "spec", Type: "string", Description: "Path to the OpenAPI/Swagger spec file.", Required: true},
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

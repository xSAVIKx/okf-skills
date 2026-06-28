package main

import "github.com/xSAVIKx/okf-skills/okf-go"

// buildSchema returns the machine-readable self-description of the okf-lint skill.
func buildSchema() okf.SkillSchema {
	return okf.SkillSchema{
		Name:        "okf-lint",
		Description: "Validate an OKF bundle for spec conformance and enrichment coverage, gating CI via exit code.",
		Commands: []okf.CommandSchema{
			{
				Name:        "lint",
				Description: "Validate a bundle (spec conformance + coverage); exit 1 on violations.",
				Flags: []okf.FlagSchema{
					{Name: "bundle", Type: "string", Description: "Path to the OKF bundle directory.", Required: true},
					{Name: "min", Type: "string", Description: "Fail if enriched % is below this threshold (0 = no gate).", Default: "0"},
					{Name: "max-broken-links", Type: "int", Description: "Maximum tolerated broken cross-links before failing.", Default: "0"},
					{Name: "require-types", Type: "bool", Description: "Fail if any concept is missing a non-empty type.", Default: "true"},
					{Name: "strict", Type: "bool", Description: "Also fail when there are orphan (cross-link-less) concepts.", Default: "false"},
					{Name: "json", Type: "bool", Description: "Emit the report as JSON instead of text.", Default: "false"},
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

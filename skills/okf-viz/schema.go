package main

import "github.com/xSAVIKx/okf-skills/okf-go"

// buildSchema returns the machine-readable self-description of the okf-viz skill.
func buildSchema() okf.SkillSchema {
	return okf.SkillSchema{
		Name:        "okf-viz",
		Description: "Render an OKF bundle into a single self-contained interactive index.html.",
		Commands: []okf.CommandSchema{
			{
				Name:        "render",
				Description: "Render an OKF bundle to a self-contained HTML visualization.",
				Flags: []okf.FlagSchema{
					{Name: "bundle", Type: "string", Description: "Path to the OKF bundle directory.", Required: true},
					{Name: "out", Type: "string", Description: "Output HTML path (default <bundle>/index.html)."},
					{Name: "offline", Type: "bool", Description: "Inline the graph library instead of using a CDN.", Default: "false"},
					{Name: "lang", Type: "string", Description: "UI-chrome language code.", Default: "en"},
					{Name: "theme", Type: "string", Description: "Initial theme: light, dark, or system.", Default: "system"},
					{Name: "title", Type: "string", Description: "Page title (default derived from the bundle)."},
					{Name: "inline-all", Type: "bool", Description: "Always inline every concept body (single file, regardless of size).", Default: "false"},
					{Name: "threshold", Type: "int", Description: "Concept count above which bodies are written as lazy sibling fragments (0 = default).", Default: "0"},
					{Name: "diff", Type: "string", Description: "Diff against another bundle: highlight added/removed/changed concepts and edges."},
					{Name: "bundles", Type: "string", Description: "Comma-separated additional bundles to federate into one cross-bundle view."},
				},
			},
			{
				Name:        "coverage",
				Description: "Report deterministic, no-LLM enrichment coverage for a bundle.",
				Flags: []okf.FlagSchema{
					{Name: "bundle", Type: "string", Description: "Path to the OKF bundle directory.", Required: true},
					{Name: "min", Type: "string", Description: "Fail (exit 1) if enriched % is below this threshold (0 = no gate).", Default: "0"},
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

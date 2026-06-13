package main

import (
	"os"

	"github.com/savikne/okf-skills-registry/okf-go"
)

// resolveAPIKey returns the explicit flag value, else OKF_LLM_API_KEY, else
// OPENAI_API_KEY. Keeps secrets out of argv when env is used.
func resolveAPIKey(flagVal string) string {
	if flagVal != "" {
		return flagVal
	}
	if v := os.Getenv("OKF_LLM_API_KEY"); v != "" {
		return v
	}
	return os.Getenv("OPENAI_API_KEY")
}

// buildSchema returns the machine-readable self-description of okf-enrich.
func buildSchema() okf.SkillSchema {
	return okf.SkillSchema{
		Name:        "okf-enrich",
		Description: "Generate concept descriptions in an OKF bundle using an OpenAI-compatible LLM (in-place enrichment).",
		Commands: []okf.CommandSchema{
			{Name: "enrich", Description: "Generate and write concept descriptions into an OKF bundle.", Flags: []okf.FlagSchema{
				{Name: "bundle", Type: "string", Description: "Path to the OKF bundle directory.", Required: true},
				{Name: "base-url", Type: "string", Description: "OpenAI-compatible API base URL.", Default: "https://api.openai.com/v1"},
				{Name: "model", Type: "string", Description: "Model name.", Default: "gpt-4o-mini"},
				{Name: "api-key", Type: "string", Description: "API key (or set OKF_LLM_API_KEY / OPENAI_API_KEY).", Env: "OKF_LLM_API_KEY"},
				{Name: "overwrite", Type: "bool", Description: "Regenerate descriptions even if already present.", Default: "false"},
			}},
			{Name: "schema", Description: "Print this skill's machine-readable JSON self-description.", Flags: []okf.FlagSchema{}},
		},
	}
}

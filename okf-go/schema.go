package okf

import (
	"encoding/json"
	"io"
)

// FlagSchema describes a single CLI flag of a skill command. It is the
// machine-readable contract a generic harness (e.g. an MCP server) uses to
// expose the skill as a tool without hardcoding its CLI surface.
type FlagSchema struct {
	Name        string `json:"name"`              // flag name without leading dashes (e.g. "db")
	Type        string `json:"type"`              // "string", "int", or "bool"
	Description string `json:"description"`       // human/LLM-facing description
	Required    bool   `json:"required"`          // whether the command refuses to run without it
	Default     string `json:"default,omitempty"` // default value rendered as text
	Env         string `json:"env,omitempty"`     // environment variable that can supply this value
}

// CommandSchema describes one subcommand (e.g. produce, ingest, schema).
type CommandSchema struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Flags       []FlagSchema `json:"flags"`
}

// SkillSchema is the top-level self-description emitted by a skill's `schema`
// subcommand.
type SkillSchema struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Commands    []CommandSchema `json:"commands"`
}

// PrintSchema writes the skill schema as indented JSON.
func PrintSchema(w io.Writer, s SkillSchema) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(s)
}

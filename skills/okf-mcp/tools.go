package main

import (
	"github.com/savikne/okf-skills-registry/okf-go"
)

// toolName builds the MCP tool name for a skill command. Skill names contain a
// hyphen (e.g. "okf-sqlite"); a double underscore separates the command so the
// skill name stays readable and reversible.
func toolName(skill, command string) string {
	return skill + "__" + command
}

// mcpType maps an okf FlagSchema type to a JSON Schema type. Unknown types
// default to "string".
func mcpType(t string) string {
	switch t {
	case "int":
		return "integer"
	case "bool":
		return "boolean"
	default:
		return "string"
	}
}

// inputSchema builds a JSON Schema (2020-12 compatible object) describing a
// command's flags, suitable for mcp.Tool.InputSchema.
func inputSchema(cmd okf.CommandSchema) map[string]any {
	props := map[string]any{}
	var required []string
	for _, f := range cmd.Flags {
		prop := map[string]any{
			"type":        mcpType(f.Type),
			"description": f.Description,
		}
		props[f.Name] = prop
		if f.Required {
			required = append(required, f.Name)
		}
	}
	schema := map[string]any{
		"type":       "object",
		"properties": props,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

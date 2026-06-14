package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/xSAVIKx/okf-skills/okf-go"
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

// buildInvocation translates MCP tool arguments into a CLI invocation for a
// skill command. It returns argv (starting with the command name) and a slice
// of "NAME=value" environment entries for flags that advertise an Env binding
// (so secrets like passwords never appear in argv). Missing required flags are
// an error. Boolean flags are emitted as a bare "--flag" when true and omitted
// when false.
func buildInvocation(cmd okf.CommandSchema, args map[string]any) (argv []string, env []string, err error) {
	argv = []string{cmd.Name}
	for _, f := range cmd.Flags {
		raw, present := args[f.Name]
		if !present {
			if f.Required {
				return nil, nil, fmt.Errorf("missing required argument %q", f.Name)
			}
			continue
		}

		if f.Type == "bool" {
			b, ok := raw.(bool)
			if !ok {
				return nil, nil, fmt.Errorf("argument %q must be a boolean", f.Name)
			}
			if b {
				argv = append(argv, "--"+f.Name)
			}
			continue
		}

		val, err := stringifyArg(f.Type, raw)
		if err != nil {
			return nil, nil, fmt.Errorf("argument %q: %w", f.Name, err)
		}
		if f.Env != "" {
			env = append(env, f.Env+"="+val)
		} else {
			argv = append(argv, "--"+f.Name, val)
		}
	}
	return argv, env, nil
}

// stringifyArg renders a JSON-decoded argument value as a CLI string according
// to the declared flag type. JSON numbers decode to float64, so integers are
// formatted without a decimal point.
func stringifyArg(flagType string, raw any) (string, error) {
	switch flagType {
	case "int":
		switch n := raw.(type) {
		case float64:
			return strconv.FormatInt(int64(n), 10), nil
		case int:
			return strconv.Itoa(n), nil
		case string:
			return n, nil
		default:
			return "", fmt.Errorf("expected a number, got %T", raw)
		}
	default: // string and unknown
		switch s := raw.(type) {
		case string:
			return s, nil
		default:
			return strings.TrimSpace(fmt.Sprintf("%v", s)), nil
		}
	}
}

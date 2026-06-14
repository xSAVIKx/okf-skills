package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/savikne/okf-skills/okf-go"
)

// Runner executes a skill binary with the given argv and extra environment
// entries, returning combined stdout. It is an injection seam so the server
// wiring can be tested without spawning real processes.
type Runner func(ctx context.Context, bin string, argv, env []string) (string, error)

// DiscoveredSkill pairs a skill binary path with its parsed self-description.
type DiscoveredSkill struct {
	Bin    string
	Schema okf.SkillSchema
}

// errorResult wraps a message as a failed tool result. Per MCP convention, tool
// execution failures are returned as IsError results (so the model can see and
// react to them), not as protocol-level errors.
func errorResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
	}
}

// registerSkills reflects every discovered skill's commands (except the
// introspection-only "schema" command) into MCP tools on the server. Each tool
// translates its arguments into a CLI invocation and runs the skill via run.
func registerSkills(server *mcp.Server, skills []DiscoveredSkill, run Runner) {
	for _, sk := range skills {
		for _, command := range sk.Schema.Commands {
			if command.Name == "schema" {
				continue
			}
			bin := sk.Bin
			cmd := command // capture per iteration
			tool := &mcp.Tool{
				Name:        toolName(sk.Schema.Name, cmd.Name),
				Description: fmt.Sprintf("[%s] %s", sk.Schema.Name, cmd.Description),
				InputSchema: inputSchema(cmd),
			}
			server.AddTool(tool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				var args map[string]any
				if len(req.Params.Arguments) > 0 {
					if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
						return errorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
					}
				}
				argv, env, err := buildInvocation(cmd, args)
				if err != nil {
					return errorResult(err.Error()), nil
				}
				out, err := run(ctx, bin, argv, env)
				if err != nil {
					text := out
					if text != "" {
						text += "\n"
					}
					return errorResult(text + err.Error()), nil
				}
				return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: out}}}, nil
			})
		}
	}
}

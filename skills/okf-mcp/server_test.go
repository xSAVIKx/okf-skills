package main

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/savikne/okf-skills-registry/okf-go"
)

func TestServerRoundTrip(t *testing.T) {
	skill := DiscoveredSkill{
		Bin: "okf-sqlite",
		Schema: okf.SkillSchema{
			Name: "okf-sqlite",
			Commands: []okf.CommandSchema{
				{Name: "produce", Description: "make", Flags: []okf.FlagSchema{
					{Name: "db", Type: "string", Required: true},
					{Name: "out", Type: "string", Required: true},
					{Name: "profile", Type: "bool"},
				}},
				{Name: "schema", Description: "introspect"},
			},
		},
	}

	var gotBin string
	var gotArgv, gotEnv []string
	run := func(ctx context.Context, bin string, argv, env []string) (string, error) {
		gotBin, gotArgv, gotEnv = bin, argv, env
		return "PRODUCED OK", nil
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "okf-mcp", Version: "test"}, nil)
	registerSkills(server, []DiscoveredSkill{skill}, run)

	clientT, serverT := mcp.NewInMemoryTransports()
	ctx := context.Background()
	ss, err := server.Connect(ctx, serverT, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer ss.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	cs, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer cs.Close()

	// ListTools: produce is exposed, schema is not.
	lt, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	names := map[string]bool{}
	for _, tl := range lt.Tools {
		names[tl.Name] = true
	}
	if !names["okf-sqlite__produce"] {
		t.Fatalf("produce tool missing; got %v", names)
	}
	if names["okf-sqlite__schema"] {
		t.Fatal("the schema command must not be exposed as a tool")
	}

	// CallTool: produce with args is translated and runner output returned.
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "okf-sqlite__produce",
		Arguments: map[string]any{"db": "x.db", "out": "o", "profile": true},
	})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error result: %v", res.Content)
	}
	if gotBin != "okf-sqlite" {
		t.Fatalf("runner bin = %q", gotBin)
	}
	if join(gotArgv) != "produce --db x.db --out o --profile" {
		t.Fatalf("runner argv = %q", join(gotArgv))
	}
	if len(gotEnv) != 0 {
		t.Fatalf("runner env = %v, want empty", gotEnv)
	}
	if len(res.Content) == 0 {
		t.Fatal("no content returned")
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok || tc.Text != "PRODUCED OK" {
		t.Fatalf("content = %#v", res.Content)
	}
}

func TestServerRoundTrip_MissingRequiredIsToolError(t *testing.T) {
	skill := DiscoveredSkill{
		Bin: "okf-sqlite",
		Schema: okf.SkillSchema{Name: "okf-sqlite", Commands: []okf.CommandSchema{
			{Name: "produce", Flags: []okf.FlagSchema{{Name: "db", Type: "string", Required: true}}},
		}},
	}
	run := func(ctx context.Context, bin string, argv, env []string) (string, error) {
		t.Fatal("runner must not be called when a required arg is missing")
		return "", nil
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "okf-mcp", Version: "test"}, nil)
	registerSkills(server, []DiscoveredSkill{skill}, run)
	clientT, serverT := mcp.NewInMemoryTransports()
	ctx := context.Background()
	ss, _ := server.Connect(ctx, serverT, nil)
	defer ss.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "c", Version: "t"}, nil)
	cs, _ := client.Connect(ctx, clientT, nil)
	defer cs.Close()

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "okf-sqlite__produce", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("call tool transport error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected IsError result for missing required arg")
	}
}

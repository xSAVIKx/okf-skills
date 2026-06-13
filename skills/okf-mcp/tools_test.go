package main

import (
	"testing"

	"github.com/savikne/okf-skills-registry/okf-go"
)

func TestToolName(t *testing.T) {
	if got := toolName("okf-sqlite", "produce"); got != "okf-sqlite__produce" {
		t.Fatalf("toolName = %q", got)
	}
}

func TestMcpType(t *testing.T) {
	cases := map[string]string{"string": "string", "int": "integer", "bool": "boolean", "weird": "string"}
	for in, want := range cases {
		if got := mcpType(in); got != want {
			t.Fatalf("mcpType(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestInputSchema(t *testing.T) {
	cmd := okf.CommandSchema{
		Name: "produce",
		Flags: []okf.FlagSchema{
			{Name: "db", Type: "string", Description: "db path", Required: true},
			{Name: "sample", Type: "int", Description: "n rows"},
			{Name: "profile", Type: "bool", Description: "profile"},
		},
	}
	s := inputSchema(cmd)
	if s["type"] != "object" {
		t.Fatalf("type = %v", s["type"])
	}
	props, ok := s["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties wrong type: %T", s["properties"])
	}
	db, _ := props["db"].(map[string]any)
	if db["type"] != "string" || db["description"] != "db path" {
		t.Fatalf("db prop = %v", db)
	}
	if sample, _ := props["sample"].(map[string]any); sample["type"] != "integer" {
		t.Fatalf("sample prop = %v", props["sample"])
	}
	if profile, _ := props["profile"].(map[string]any); profile["type"] != "boolean" {
		t.Fatalf("profile prop = %v", props["profile"])
	}
	req, ok := s["required"].([]string)
	if !ok || len(req) != 1 || req[0] != "db" {
		t.Fatalf("required = %v", s["required"])
	}
}

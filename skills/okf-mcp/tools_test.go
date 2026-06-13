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

func TestBuildInvocation_BasicAndBoolAndInt(t *testing.T) {
	cmd := okf.CommandSchema{
		Name: "produce",
		Flags: []okf.FlagSchema{
			{Name: "db", Type: "string", Required: true},
			{Name: "out", Type: "string", Required: true},
			{Name: "tables", Type: "string"},
			{Name: "sample", Type: "int"},
			{Name: "profile", Type: "bool"},
		},
	}
	// sample arrives as float64 (JSON number); profile true → bare flag; tables omitted.
	argv, env, err := buildInvocation(cmd, map[string]any{
		"db": "x.db", "out": "o", "sample": float64(5), "profile": true,
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(env) != 0 {
		t.Fatalf("env = %v, want empty", env)
	}
	got := join(argv)
	want := "produce --db x.db --out o --sample 5 --profile"
	if got != want {
		t.Fatalf("argv = %q, want %q", got, want)
	}
}

func TestBuildInvocation_MissingRequired(t *testing.T) {
	cmd := okf.CommandSchema{Name: "produce", Flags: []okf.FlagSchema{
		{Name: "db", Type: "string", Required: true},
	}}
	if _, _, err := buildInvocation(cmd, map[string]any{}); err == nil {
		t.Fatal("expected error for missing required flag")
	}
}

func TestBuildInvocation_BoolFalseOmitted(t *testing.T) {
	cmd := okf.CommandSchema{Name: "ingest", Flags: []okf.FlagSchema{
		{Name: "bundle", Type: "string", Required: true},
		{Name: "sync", Type: "bool"},
	}}
	argv, _, err := buildInvocation(cmd, map[string]any{"bundle": "b", "sync": false})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if join(argv) != "ingest --bundle b" {
		t.Fatalf("argv = %q", join(argv))
	}
}

func TestBuildInvocation_EnvFlagRoutedToEnvNotArgv(t *testing.T) {
	cmd := okf.CommandSchema{Name: "produce", Flags: []okf.FlagSchema{
		{Name: "user", Type: "string", Required: true},
		{Name: "password", Type: "string", Required: true, Env: "MYSQL_PASSWORD"},
		{Name: "out", Type: "string", Required: true},
	}}
	argv, env, err := buildInvocation(cmd, map[string]any{
		"user": "root", "password": "secret", "out": "o",
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if join(argv) != "produce --user root --out o" {
		t.Fatalf("argv leaked password or wrong: %q", join(argv))
	}
	if len(env) != 1 || env[0] != "MYSQL_PASSWORD=secret" {
		t.Fatalf("env = %v, want [MYSQL_PASSWORD=secret]", env)
	}
}

func join(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += " "
		}
		out += p
	}
	return out
}

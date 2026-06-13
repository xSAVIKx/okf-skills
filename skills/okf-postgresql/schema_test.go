package main

import (
	"os"
	"testing"
)

func TestBuildSchema(t *testing.T) {
	s := buildSchema()
	if s.Name != "okf-postgresql" {
		t.Fatalf("name = %q", s.Name)
	}
	cmds := map[string]bool{}
	for _, c := range s.Commands {
		cmds[c.Name] = true
	}
	for _, want := range []string{"produce", "ingest", "schema"} {
		if !cmds[want] {
			t.Fatalf("missing command %q", want)
		}
	}
	var foundSchemaFlag, foundEnv bool
	for _, c := range s.Commands {
		for _, f := range c.Flags {
			if f.Name == "schema" {
				foundSchemaFlag = true
			}
			if f.Name == "password" && f.Env == "PGPASSWORD" {
				foundEnv = true
			}
		}
	}
	if !foundSchemaFlag {
		t.Fatal("expected a 'schema' flag (postgres schema name)")
	}
	if !foundEnv {
		t.Fatal("password flag must advertise env PGPASSWORD")
	}
}

func TestResolvePassword(t *testing.T) {
	t.Setenv("PGPASSWORD", "fromenv")
	if got := resolvePassword(""); got != "fromenv" {
		t.Fatalf("empty flag should fall back to env, got %q", got)
	}
	if got := resolvePassword("explicit"); got != "explicit" {
		t.Fatalf("explicit flag should win, got %q", got)
	}
	os.Unsetenv("PGPASSWORD")
}

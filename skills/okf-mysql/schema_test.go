package main

import (
	"os"
	"testing"
)

func TestBuildSchema(t *testing.T) {
	s := buildSchema()
	if s.Name != "okf-mysql" {
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
	// password flag must advertise its env binding
	var foundEnv bool
	for _, c := range s.Commands {
		for _, f := range c.Flags {
			if f.Name == "password" && f.Env == "MYSQL_PASSWORD" {
				foundEnv = true
			}
		}
	}
	if !foundEnv {
		t.Fatal("password flag must advertise env MYSQL_PASSWORD")
	}
}

func TestResolvePassword(t *testing.T) {
	t.Setenv("MYSQL_PASSWORD", "fromenv")
	if got := resolvePassword(""); got != "fromenv" {
		t.Fatalf("empty flag should fall back to env, got %q", got)
	}
	if got := resolvePassword("explicit"); got != "explicit" {
		t.Fatalf("explicit flag should win, got %q", got)
	}
	os.Unsetenv("MYSQL_PASSWORD")
}

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

func TestParseSchema(t *testing.T) {
	data := []byte(`{"name":"okf-sqlite","description":"d","commands":[{"name":"produce","description":"p","flags":[{"name":"db","type":"string","required":true}]}]}`)
	s, err := parseSchema(data)
	if err != nil {
		t.Fatalf("parseSchema: %v", err)
	}
	if s.Name != "okf-sqlite" || len(s.Commands) != 1 || s.Commands[0].Flags[0].Name != "db" {
		t.Fatalf("parsed wrong: %+v", s)
	}
}

func TestParseSchema_Invalid(t *testing.T) {
	if _, err := parseSchema([]byte("not json")); err == nil {
		t.Fatal("expected error on invalid JSON")
	}
}

func TestDiscoverSkills(t *testing.T) {
	dir := t.TempDir()
	// Create fake executables; only okf-* (excluding the server itself) should be found.
	for _, name := range []string{"okf-sqlite", "okf-mysql", "okf-mcp", "ls"} {
		path := filepath.Join(dir, exeName(name))
		if err := os.WriteFile(path, []byte("x"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	got := discoverSkills([]string{dir}, "okf-mcp")
	var names []string
	for _, p := range got {
		names = append(names, strings.TrimSuffix(filepath.Base(p), exeSuffix()))
	}
	sort.Strings(names)
	if len(names) != 2 || names[0] != "okf-mysql" || names[1] != "okf-sqlite" {
		t.Fatalf("discovered = %v, want [okf-mysql okf-sqlite]", names)
	}
	_ = runtime.GOOS
}

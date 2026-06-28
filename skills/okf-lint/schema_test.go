package main

import "testing"

func TestBuildSchema(t *testing.T) {
	s := buildSchema()
	if s.Name != "okf-lint" {
		t.Fatalf("name = %q", s.Name)
	}
	cmds := map[string]bool{}
	for _, c := range s.Commands {
		cmds[c.Name] = true
	}
	for _, want := range []string{"lint", "schema"} {
		if !cmds[want] {
			t.Fatalf("missing command %q", want)
		}
	}
	// lint must declare a required bundle flag
	for _, c := range s.Commands {
		if c.Name != "lint" {
			continue
		}
		req := map[string]bool{}
		for _, f := range c.Flags {
			if f.Required {
				req[f.Name] = true
			}
		}
		if !req["bundle"] {
			t.Fatalf("lint required flags = %v, want bundle", req)
		}
	}
}

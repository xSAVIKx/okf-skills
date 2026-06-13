package main

import "testing"

func TestBuildSchema(t *testing.T) {
	s := buildSchema()
	if s.Name != "okf-bigquery" {
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
	for _, c := range s.Commands {
		if c.Name != "produce" {
			continue
		}
		req := map[string]bool{}
		for _, f := range c.Flags {
			if f.Required {
				req[f.Name] = true
			}
		}
		if !req["project"] || !req["dataset"] || !req["out"] {
			t.Fatalf("produce required flags = %v, want project+dataset+out", req)
		}
	}
}

package main

import "testing"

func TestBuildSchema(t *testing.T) {
	s := buildSchema()
	if s.Name != "okf-viz" {
		t.Fatalf("Name = %q, want okf-viz", s.Name)
	}
	cmds := map[string]bool{}
	for _, c := range s.Commands {
		cmds[c.Name] = true
	}
	for _, want := range []string{"render", "schema"} {
		if !cmds[want] {
			t.Errorf("missing command %q", want)
		}
	}
	// render must advertise the required --bundle flag
	var render *struct{ found, bundleRequired bool }
	render = &struct{ found, bundleRequired bool }{}
	for _, c := range s.Commands {
		if c.Name != "render" {
			continue
		}
		render.found = true
		for _, f := range c.Flags {
			if f.Name == "bundle" && f.Required {
				render.bundleRequired = true
			}
		}
	}
	if !render.found || !render.bundleRequired {
		t.Errorf("render must declare a required --bundle flag; got %+v", render)
	}
}

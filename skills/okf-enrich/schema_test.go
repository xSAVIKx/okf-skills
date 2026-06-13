package main

import "testing"

func TestBuildSchema(t *testing.T) {
	s := buildSchema()
	if s.Name != "okf-enrich" {
		t.Fatalf("name=%q", s.Name)
	}
	cmds := map[string]bool{}
	for _, c := range s.Commands {
		cmds[c.Name] = true
	}
	if !cmds["enrich"] || !cmds["schema"] {
		t.Fatalf("missing commands: %v", cmds)
	}
	var bundleReq, keyEnv bool
	for _, c := range s.Commands {
		if c.Name != "enrich" {
			continue
		}
		for _, f := range c.Flags {
			if f.Name == "bundle" && f.Required {
				bundleReq = true
			}
			if f.Name == "api-key" && f.Env == "OKF_LLM_API_KEY" {
				keyEnv = true
			}
		}
	}
	if !bundleReq {
		t.Fatal("enrich must require bundle")
	}
	if !keyEnv {
		t.Fatal("api-key must advertise env OKF_LLM_API_KEY")
	}
}

func TestResolveAPIKey(t *testing.T) {
	t.Setenv("OKF_LLM_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	if resolveAPIKey("flag") != "flag" {
		t.Fatal("explicit flag should win")
	}
	t.Setenv("OKF_LLM_API_KEY", "envkey")
	if resolveAPIKey("") != "envkey" {
		t.Fatal("OKF_LLM_API_KEY fallback")
	}
	t.Setenv("OKF_LLM_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "openaikey")
	if resolveAPIKey("") != "openaikey" {
		t.Fatal("OPENAI_API_KEY fallback")
	}
}

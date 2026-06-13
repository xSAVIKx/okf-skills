package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDirs(t *testing.T) {
	// An explicit --skills-dir flag always wins.
	if got := resolveDirs("/explicit/dir"); len(got) != 1 || got[0] != "/explicit/dir" {
		t.Fatalf("explicit flag: got %v, want [/explicit/dir]", got)
	}

	// With no flag, OKF_SKILLS_DIR is used.
	t.Setenv("OKF_SKILLS_DIR", "/env/dir")
	if got := resolveDirs(""); len(got) != 1 || got[0] != "/env/dir" {
		t.Fatalf("env fallback: got %v, want [/env/dir]", got)
	}

	// With neither flag nor OKF_SKILLS_DIR, fall back to PATH entries.
	t.Setenv("OKF_SKILLS_DIR", "")
	pathVal := "dirA" + string(os.PathListSeparator) + "dirB"
	t.Setenv("PATH", pathVal)
	got := resolveDirs("")
	want := filepath.SplitList(pathVal)
	if len(got) != len(want) || got[0] != "dirA" || got[len(got)-1] != "dirB" {
		t.Fatalf("PATH fallback: got %v, want %v", got, want)
	}
}

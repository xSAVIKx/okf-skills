package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// findRepoRoot walks up from the current working directory until it finds the
// directory containing go.work (the repo root). This lets the tool be invoked from
// anywhere in the tree (e.g. `go run ./tools/scaffold` from root, or via make).
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not locate repo root (no go.work found in any parent directory)")
		}
		dir = parent
	}
}

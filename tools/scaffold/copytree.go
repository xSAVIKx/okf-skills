package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// copyTree recursively copies the reference skill directory to dst, skipping built
// binaries (the reference's compiled `okf-<ref>` / `okf-<ref>.exe`) so the new
// skeleton starts source-only.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		if skipFile(d.Name()) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		// A new skill starts with an empty changelog; release-please populates it on
		// first release. Copying the reference's would inherit its bogus version history.
		if d.Name() == "CHANGELOG.md" {
			data = []byte("# Changelog\n")
		}
		out := filepath.Join(dst, rel)
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		return os.WriteFile(out, data, 0o644)
	})
}

// skipFile reports whether a file should not be copied: compiled binaries only.
// go.sum IS copied — slug-replace changes only the module path, not its deps, so the
// reference's go.sum stays valid and the skeleton builds green with no `go mod tidy`.
func skipFile(name string) bool {
	if strings.HasSuffix(name, ".exe") {
		return true
	}
	// A binary built in place is named exactly like the reference skill dir
	// (e.g. "okf-sqlite"); those carry no extension. Skip extension-less files
	// whose name starts with "okf-" and that aren't source/markdown.
	if strings.HasPrefix(name, "okf-") && filepath.Ext(name) == "" {
		return true
	}
	return false
}

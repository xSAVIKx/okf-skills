package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/savikne/okf-skills/okf-go"
)

// exeSuffix returns the platform executable suffix ("" on Unix, ".exe" on Windows).
func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// exeName returns the on-disk executable filename for a skill base name.
func exeName(skill string) string {
	return skill + exeSuffix()
}

// discoverSkills scans the given directories for "okf-*" executables, excluding
// the named skill (the server itself). It returns full paths, de-duplicated by
// skill base name (first directory wins). Whether each candidate is really a
// skill is decided later by loadSchema (which skips anything without a working
// `schema` command).
func discoverSkills(dirs []string, exclude string) []string {
	seen := map[string]bool{}
	var paths []string
	suffix := exeSuffix()
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			base := e.Name()
			if suffix != "" && !strings.HasSuffix(base, suffix) {
				continue
			}
			name := strings.TrimSuffix(base, suffix)
			if !strings.HasPrefix(name, "okf-") || name == exclude {
				continue
			}
			if seen[name] {
				continue
			}
			seen[name] = true
			paths = append(paths, filepath.Join(dir, base))
		}
	}
	return paths
}

// parseSchema decodes a skill's `schema` JSON output into an okf.SkillSchema.
func parseSchema(data []byte) (okf.SkillSchema, error) {
	var s okf.SkillSchema
	if err := json.Unmarshal(data, &s); err != nil {
		return okf.SkillSchema{}, fmt.Errorf("parse schema: %w", err)
	}
	if s.Name == "" {
		return okf.SkillSchema{}, fmt.Errorf("schema has empty name")
	}
	return s, nil
}

// loadSchema runs "<bin> schema" and parses its stdout. It is the seam between
// discovery and the pure parser; a binary that is not an OKF skill (no schema
// command) returns an error and is skipped by the caller.
func loadSchema(ctx context.Context, bin string) (okf.SkillSchema, error) {
	cmd := exec.CommandContext(ctx, bin, "schema")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return okf.SkillSchema{}, fmt.Errorf("%s schema: %v: %s", bin, err, strings.TrimSpace(stderr.String()))
	}
	return parseSchema(stdout.Bytes())
}

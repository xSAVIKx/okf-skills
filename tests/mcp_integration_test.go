package tests

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// skillSchema is a minimal mirror of okf.SkillSchema used to validate the JSON
// self-description contract without importing the okf-go module into this test.
type skillSchema struct {
	Name     string `json:"name"`
	Commands []struct {
		Name string `json:"name"`
	} `json:"commands"`
}

// TestConnectorSchemaContract verifies that every built connector binary emits a
// valid `schema` JSON self-description carrying the expected name and commands —
// the exact contract okf-mcp relies on to discover and expose skills as tools.
func TestConnectorSchemaContract(t *testing.T) {
	connectors := []string{
		"okf-sqlite", "okf-mysql", "okf-postgresql",
		"okf-bigquery", "okf-fs", "okf-git",
	}
	for _, skill := range connectors {
		t.Run(skill, func(t *testing.T) {
			bin := getBinaryPath(skill)
			if _, err := os.Stat(bin); os.IsNotExist(err) {
				t.Skipf("%s not built at %s (run 'make build' or skills.sh first)", skill, bin)
			}
			out, err := exec.Command(bin, "schema").Output()
			if err != nil {
				t.Fatalf("%s schema failed: %v", skill, err)
			}
			var s skillSchema
			if err := json.Unmarshal(out, &s); err != nil {
				t.Fatalf("%s schema is not valid JSON: %v\n%s", skill, err, out)
			}
			if s.Name != skill {
				t.Errorf("%s schema name = %q, want %q", skill, s.Name, skill)
			}
			have := map[string]bool{}
			for _, c := range s.Commands {
				have[c.Name] = true
			}
			for _, want := range []string{"produce", "ingest", "schema"} {
				if !have[want] {
					t.Errorf("%s schema missing command %q (have %v)", skill, want, have)
				}
			}
		})
	}
}

// mcpBinaryPath returns the path to the okf-mcp server binary (a top-level
// module, not under skills/).
func mcpBinaryPath() string {
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	return filepath.Join("..", "okf-mcp", "okf-mcp"+ext)
}

// TestMCPDiscovery verifies the okf-mcp server discovers a real connector binary
// end-to-end: it scans a skills directory, runs each binary's `schema` command,
// and registers it. This automates the manual discovery smoke test.
func TestMCPDiscovery(t *testing.T) {
	mcpBin := mcpBinaryPath()
	if _, err := os.Stat(mcpBin); os.IsNotExist(err) {
		t.Skipf("okf-mcp not built at %s (run 'make build' first)", mcpBin)
	}
	sqliteBin := getBinaryPath("okf-sqlite")
	if _, err := os.Stat(sqliteBin); os.IsNotExist(err) {
		t.Skipf("okf-sqlite not built at %s", sqliteBin)
	}

	// Stage the connector binary in an isolated skills directory.
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	skillsDir := t.TempDir()
	data, err := os.ReadFile(sqliteBin)
	if err != nil {
		t.Fatalf("read sqlite binary: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillsDir, "okf-sqlite"+ext), data, 0o755); err != nil {
		t.Fatalf("stage sqlite binary: %v", err)
	}

	// okf-mcp logs discovery to stderr, then blocks on the stdio MCP transport.
	// Feed it empty stdin (immediate EOF) so it exits, and bound it with a
	// timeout so the test can never hang. We assert on the discovery log, not
	// the exit status (which depends on how the stdio session closes).
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, mcpBin, "--skills-dir", skillsDir)
	cmd.Stdin = strings.NewReader("")
	var stderr bytesBuffer
	cmd.Stderr = &stderr
	_ = cmd.Run()

	if !strings.Contains(stderr.String(), "registered skill okf-sqlite") {
		t.Fatalf("okf-mcp did not register okf-sqlite; stderr:\n%s", stderr.String())
	}
}

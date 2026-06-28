// Command okf-mcp is a generic MCP server that exposes installed okf-* skills
// as MCP tools by reading each skill's `schema` self-description. It speaks MCP
// over stdio; all diagnostics go to stderr (stdout is the protocol channel).
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// version is the build version, injected via -ldflags "-X main.version=..." by
// install.sh; it defaults to "dev" for plain `go build`.
var version = "dev"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "version" || os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println(version)
		return
	}

	skillsDir := flag.String("skills-dir", "", "directory to scan for okf-* skills (default: $OKF_SKILLS_DIR, else each $PATH entry)")
	timeout := flag.Duration("timeout", 5*time.Minute, "per-invocation timeout for skill commands")
	flag.Parse()

	ctx := context.Background()
	dirs := resolveDirs(*skillsDir)

	var skills []DiscoveredSkill
	for _, path := range discoverSkills(dirs, "okf-mcp") {
		schema, err := loadSchema(ctx, path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "okf-mcp: skipping %s: %v\n", path, err)
			continue
		}
		skills = append(skills, DiscoveredSkill{Bin: path, Schema: schema})
		fmt.Fprintf(os.Stderr, "okf-mcp: registered skill %s (%s)\n", schema.Name, path)
	}
	if len(skills) == 0 {
		fmt.Fprintln(os.Stderr, "okf-mcp: warning: no okf-* skills discovered")
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "okf-mcp", Version: version}, nil)
	registerSkills(server, skills, execRunner(*timeout))

	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		fmt.Fprintf(os.Stderr, "okf-mcp: server error: %v\n", err)
		os.Exit(1)
	}
}

// resolveDirs decides which directories to scan: the --skills-dir flag if set,
// else $OKF_SKILLS_DIR, else every entry on $PATH.
func resolveDirs(flagDir string) []string {
	if flagDir != "" {
		return []string{flagDir}
	}
	if env := os.Getenv("OKF_SKILLS_DIR"); env != "" {
		return []string{env}
	}
	return filepath.SplitList(os.Getenv("PATH"))
}

// execRunner returns a Runner that executes skill binaries with a per-call
// timeout, inheriting the process environment plus any extra entries (e.g. a DB
// password routed via env instead of argv).
func execRunner(timeout time.Duration) Runner {
	return func(ctx context.Context, bin string, argv, env []string) (string, error) {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, bin, argv...)
		cmd.Env = append(os.Environ(), env...)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return stdout.String(), fmt.Errorf("%v: %s", err, strings.TrimSpace(stderr.String()))
		}
		return stdout.String(), nil
	}
}

// Package main implements okf-viz, a consumer skill that renders an OKF bundle
// into a single self-contained interactive index.html.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/xSAVIKx/okf-skills/okf-go"
)

// version is the build version, injected via -ldflags "-X main.version=..." by
// install.sh; it defaults to "dev" for plain `go build`.
var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "render":
		runRender(os.Args[2:])
	case "coverage":
		runCoverage(os.Args[2:])
	case "schema":
		if err := okf.PrintSchema(os.Stdout, buildSchema()); err != nil {
			log.Fatalf("Failed to print schema: %v", err)
		}
	case "version", "--version", "-v":
		fmt.Println(version)
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: okf-viz <command> [options]")
	fmt.Println("Commands:")
	fmt.Println("  render    - Render an OKF bundle to a self-contained index.html")
	fmt.Println("  coverage  - Report deterministic enrichment coverage for a bundle")
	fmt.Println("  schema    - Print this skill's machine-readable JSON self-description")
	fmt.Println("\nRun 'okf-viz render -h' for options.")
}

// runCoverage implements the 'coverage' subcommand: a deterministic, no-LLM,
// read-only enrichment coverage report with an optional CI gating threshold.
func runCoverage(args []string) {
	fs := flag.NewFlagSet("coverage", flag.ExitOnError)
	bundle := fs.String("bundle", "", "Path to the OKF bundle directory (required)")
	minPct := fs.Float64("min", 0, "Fail (exit 1) if enriched %% is below this threshold (0 = no gate)")
	asJSON := fs.Bool("json", false, "Emit the report as JSON instead of text")
	_ = fs.Parse(args)

	if *bundle == "" {
		fs.Usage()
		os.Exit(1)
	}
	m, err := BuildModel(*bundle)
	if err != nil {
		log.Fatalf("Failed to read bundle: %v", err)
	}
	addCrossLinks(m)
	cov := ComputeCoverage(m)

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(cov); err != nil {
			log.Fatalf("Failed to encode report: %v", err)
		}
	} else {
		fmt.Print(cov.Report())
	}

	if *minPct > 0 && cov.EnrichedPct < *minPct {
		fmt.Fprintf(os.Stderr, "coverage gate failed: %.1f%% enriched < %.1f%% required\n", cov.EnrichedPct, *minPct)
		os.Exit(1)
	}
}

func runRender(args []string) {
	fs := flag.NewFlagSet("render", flag.ExitOnError)
	bundle := fs.String("bundle", "", "Path to the OKF bundle directory (required)")
	out := fs.String("out", "", "Output HTML path (default <bundle>/index.html)")
	offline := fs.Bool("offline", false, "Inline the graph library instead of CDN")
	lang := fs.String("lang", "en", "UI-chrome language code")
	theme := fs.String("theme", "system", "Initial theme: light, dark, or system")
	title := fs.String("title", "", "Page title (default derived from bundle)")
	inlineAll := fs.Bool("inline-all", false, "Always inline every concept body (single file, regardless of size)")
	threshold := fs.Int("threshold", 0, "Concept count above which bodies are written as lazy sibling fragments (0 = default)")
	diff := fs.String("diff", "", "Diff against another bundle: highlight added/removed/changed concepts and edges")
	bundles := fs.String("bundles", "", "Comma-separated additional bundles to federate into one cross-bundle view")
	_ = fs.Parse(args)

	if *bundle == "" {
		fs.Usage()
		os.Exit(1)
	}
	m, err := BuildModel(*bundle)
	if err != nil {
		log.Fatalf("Failed to read bundle: %v", err)
	}
	addCrossLinks(m)

	// Diff mode: annotate against an older bundle (content_hash-based, body fallback).
	if *diff != "" {
		other, derr := BuildModel(*diff)
		if derr != nil {
			log.Fatalf("Failed to read diff bundle %s: %v", *diff, derr)
		}
		addCrossLinks(other)
		ComputeDiff(m, other)
	}

	// Federation: namespace and merge additional bundles into one view.
	if *bundles != "" {
		others := map[string]*Model{}
		for _, b := range strings.Split(*bundles, ",") {
			b = strings.TrimSpace(b)
			if b == "" {
				continue
			}
			om, oerr := BuildModel(b)
			if oerr != nil {
				log.Fatalf("Failed to read federated bundle %s: %v", b, oerr)
			}
			addCrossLinks(om)
			others[filepath.Base(b)] = om
		}
		Federate(filepath.Base(*bundle), m, others)
	}

	pageTitle := *title
	if pageTitle == "" {
		pageTitle = m.RootTitle
	}
	html, fragments, err := Emit(m, EmitOptions{
		Title: pageTitle, Theme: *theme, Offline: *offline, Lang: *lang,
		InlineAll: *inlineAll, Threshold: *threshold,
	})
	if err != nil {
		log.Fatalf("Failed to render: %v", err)
	}
	outPath := *out
	if outPath == "" {
		outPath = filepath.Join(*bundle, "index.html")
	}
	if err := os.WriteFile(outPath, []byte(html), 0644); err != nil {
		log.Fatalf("Failed to write %s: %v", outPath, err)
	}
	// Lazy mode: write the per-concept body fragments next to index.html. Filenames
	// are deterministic and sorted, so re-running yields byte-identical output.
	outDir := filepath.Dir(outPath)
	fragPaths := make([]string, 0, len(fragments))
	for rel := range fragments {
		fragPaths = append(fragPaths, rel)
	}
	sort.Strings(fragPaths)
	for _, rel := range fragPaths {
		fp := filepath.Join(outDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(fp), 0755); err != nil {
			log.Fatalf("Failed to create fragment dir: %v", err)
		}
		if err := os.WriteFile(fp, []byte(fragments[rel]), 0644); err != nil {
			log.Fatalf("Failed to write fragment %s: %v", fp, err)
		}
	}
	mode := "single-file"
	if len(fragments) > 0 {
		mode = fmt.Sprintf("lazy (%d fragments)", len(fragments))
	}
	fmt.Printf("Rendered %d concepts to %s [%s]\n", len(m.concepts), outPath, mode)
}

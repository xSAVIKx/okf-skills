// Package main implements okf-lint, a consumer skill that validates an OKF bundle
// for spec conformance and enrichment coverage and gates CI with its exit code.
//
// It is deterministic and read-only (no LLM, no mutation): it delegates all scanning
// to okf-go's shared ScanBundle (the same scanner okf-viz's `coverage` command uses)
// and turns the report into pass/fail per the configured thresholds. It complements
// `skills-ref validate` (which only checks SKILL.md frontmatter) by validating the
// bundle's own conformance and documentation completeness.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/xSAVIKx/okf-skills/okf-go"
)

// version is the build version, injected via -ldflags by install.sh.
var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "lint":
		runLint(os.Args[2:])
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
	fmt.Println("Usage: okf-lint <command> [options]")
	fmt.Println("Commands:")
	fmt.Println("  lint    - Validate an OKF bundle (spec conformance + coverage); exit 1 on violations")
	fmt.Println("  schema  - Print this skill's machine-readable JSON self-description")
	fmt.Println("\nRun 'okf-lint lint -h' for options.")
}

// gateOpts are the configurable lint gates.
type gateOpts struct {
	minPct       float64
	maxBroken    int
	requireTypes bool
	strict       bool
}

// gateFailures returns a human-readable reason for each violated gate (empty = pass).
// Pure function over the report so the gating policy is unit-testable.
func gateFailures(rep *okf.LintReport, o gateOpts) []string {
	var fails []string
	structural, typeIssues := 0, 0
	for _, f := range rep.Conformance {
		if f.Rule == okf.RuleMissingType {
			typeIssues++
		} else {
			structural++
		}
	}
	if structural > 0 {
		fails = append(fails, fmt.Sprintf("%d spec-conformance violation(s)", structural))
	}
	if o.requireTypes && typeIssues > 0 {
		fails = append(fails, fmt.Sprintf("%d concept(s) missing a type", typeIssues))
	}
	if len(rep.BrokenLinks) > o.maxBroken {
		fails = append(fails, fmt.Sprintf("%d broken cross-link(s) (max %d)", len(rep.BrokenLinks), o.maxBroken))
	}
	if o.minPct > 0 && rep.EnrichedPct < o.minPct {
		fails = append(fails, fmt.Sprintf("%.1f%% enriched < %.1f%% required", rep.EnrichedPct, o.minPct))
	}
	if o.strict && len(rep.Orphans) > 0 {
		fails = append(fails, fmt.Sprintf("%d orphan concept(s) with no cross-links", len(rep.Orphans)))
	}
	return fails
}

func runLint(args []string) {
	fs := flag.NewFlagSet("lint", flag.ExitOnError)
	bundle := fs.String("bundle", "", "Path to the OKF bundle directory (required)")
	minPct := fs.Float64("min", 0, "Fail if enriched %% is below this threshold (0 = no gate)")
	maxBroken := fs.Int("max-broken-links", 0, "Maximum tolerated broken cross-links before failing")
	requireTypes := fs.Bool("require-types", true, "Fail if any concept is missing a non-empty type")
	strict := fs.Bool("strict", false, "Also fail when there are orphan (cross-link-less) concepts")
	asJSON := fs.Bool("json", false, "Emit the report as JSON instead of text")
	_ = fs.Parse(args)

	if *bundle == "" {
		fs.Usage()
		os.Exit(1)
	}
	rep, err := okf.ScanBundle(*bundle)
	if err != nil {
		log.Fatalf("Failed to scan bundle: %v", err)
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rep); err != nil {
			log.Fatalf("Failed to encode report: %v", err)
		}
	} else {
		fmt.Print(rep.TextReport())
	}

	fails := gateFailures(rep, gateOpts{
		minPct: *minPct, maxBroken: *maxBroken, requireTypes: *requireTypes, strict: *strict,
	})
	if len(fails) > 0 {
		fmt.Fprintln(os.Stderr, "lint failed:")
		for _, f := range fails {
			fmt.Fprintf(os.Stderr, "  - %s\n", f)
		}
		os.Exit(1)
	}
}

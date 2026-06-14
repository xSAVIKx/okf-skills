// Package main implements okf-viz, a consumer skill that renders an OKF bundle
// into a single self-contained interactive index.html.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/xSAVIKx/okf-skills/okf-go"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "render":
		runRender(os.Args[2:])
	case "schema":
		if err := okf.PrintSchema(os.Stdout, buildSchema()); err != nil {
			log.Fatalf("Failed to print schema: %v", err)
		}
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: okf-viz <command> [options]")
	fmt.Println("Commands:")
	fmt.Println("  render  - Render an OKF bundle to a self-contained index.html")
	fmt.Println("  schema  - Print this skill's machine-readable JSON self-description")
	fmt.Println("\nRun 'okf-viz render -h' for options.")
}

func runRender(args []string) {
	fs := flag.NewFlagSet("render", flag.ExitOnError)
	bundle := fs.String("bundle", "", "Path to the OKF bundle directory (required)")
	out := fs.String("out", "", "Output HTML path (default <bundle>/index.html)")
	offline := fs.Bool("offline", false, "Inline the graph library instead of CDN")
	lang := fs.String("lang", "en", "UI-chrome language code")
	theme := fs.String("theme", "system", "Initial theme: light, dark, or system")
	title := fs.String("title", "", "Page title (default derived from bundle)")
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

	pageTitle := *title
	if pageTitle == "" {
		pageTitle = m.RootTitle
	}
	html, err := Emit(m, EmitOptions{Title: pageTitle, Theme: *theme, Offline: *offline, Lang: *lang})
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
	fmt.Printf("Rendered %d concepts to %s\n", len(m.concepts), outPath)
}

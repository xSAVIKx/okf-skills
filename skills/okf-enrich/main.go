// Command okf-enrich generates concept descriptions in an OKF bundle using an
// OpenAI-compatible LLM, writing them back into the bundle in place.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/savikne/okf-skills-registry/okf-go"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "enrich":
		runEnrich(os.Args[2:])
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
	fmt.Println("Usage: okf-enrich <command> [options]")
	fmt.Println("Commands:")
	fmt.Println("  enrich   - Generate concept descriptions in an OKF bundle via an LLM")
	fmt.Println("  schema   - Print machine-readable JSON self-description")
}

func runEnrich(args []string) {
	fs := flag.NewFlagSet("enrich", flag.ExitOnError)
	bundle := fs.String("bundle", "", "OKF bundle directory (required)")
	baseURL := fs.String("base-url", "https://api.openai.com/v1", "OpenAI-compatible API base URL")
	model := fs.String("model", "gpt-4o-mini", "Model name")
	apiKey := fs.String("api-key", "", "API key (or set OKF_LLM_API_KEY / OPENAI_API_KEY)")
	overwrite := fs.Bool("overwrite", false, "Regenerate existing descriptions")
	fs.Parse(args)

	if *bundle == "" {
		fs.Usage()
		os.Exit(1)
	}
	key := resolveAPIKey(*apiKey)
	if key == "" {
		log.Fatal("No API key: set --api-key or the OKF_LLM_API_KEY / OPENAI_API_KEY environment variable")
	}

	gen := NewOpenAIGenerator(*baseURL, *model, key)
	n, err := enrichBundle(context.Background(), *bundle, gen, *overwrite)
	if err != nil {
		log.Fatalf("Enrichment failed: %v", err)
	}
	fmt.Printf("Enriched %d concept(s) in %s\n", n, *bundle)
}

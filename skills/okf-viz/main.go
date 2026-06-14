// Package main implements okf-viz, a consumer skill that renders an OKF bundle
// into a single self-contained interactive index.html.
package main

import (
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

// runRender is fleshed out in Task 7; stub keeps the build green for early tasks.
func runRender(args []string) {
	fs := flag.NewFlagSet("render", flag.ExitOnError)
	_ = fs.Parse(args)
	log.Fatal("render not yet implemented")
}

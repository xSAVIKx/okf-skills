// Package main implements the GraphQL OKF (Open Knowledge Format) connector. It parses
// a GraphQL SDL document into one concept per user-defined type and per root operation
// (query/mutation/subscription field), links them to the types they reference, and
// syncs descriptions back to a .okf-metadata.yaml sidecar next to the schema.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	case "produce":
		runProduce(os.Args[2:])
	case "ingest":
		runIngest(os.Args[2:])
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
	fmt.Println("Usage: okf-graphql <command> [options]")
	fmt.Println("Commands:")
	fmt.Println("  produce  - Create an OKF bundle from a GraphQL SDL document")
	fmt.Println("  ingest   - Verify a bundle against the schema; --sync writes descriptions to .okf-metadata.yaml")
}

// placeholderFor returns the deterministic placeholder description for a concept,
// e.g. "GraphQL type Order" or "GraphQL query orders".
func placeholderFor(c concept) string {
	kind := strings.ToLower(strings.TrimPrefix(c.typ, "GraphQL "))
	return "GraphQL " + kind + " " + c.title
}

func runProduce(args []string) {
	fsSet := flag.NewFlagSet("produce", flag.ExitOnError)
	schemaPath := fsSet.String("schema", "", "Path to the GraphQL SDL file (required)")
	outDir := fsSet.String("out", "", "Output bundle directory (required)")
	fsSet.Parse(args)

	if *schemaPath == "" || *outDir == "" {
		fsSet.Usage()
		os.Exit(1)
	}
	absSchema, err := filepath.Abs(*schemaPath)
	if err != nil {
		log.Fatalf("Failed to resolve schema path: %v", err)
	}
	schema, err := loadSchema(absSchema)
	if err != nil {
		log.Fatalf("Failed to load GraphQL schema: %v", err)
	}
	schemaDir := filepath.Dir(absSchema)
	meta, err := okf.ReadFolderMetadata(schemaDir)
	if err != nil {
		log.Fatalf("Failed to read folder metadata: %v", err)
	}

	concepts := extractConcepts(schema)
	timestamp := time.Now().Format(time.RFC3339)
	today := time.Now().Format("2006-01-02")
	schemaBase := filepath.Base(absSchema)

	for _, c := range concepts {
		bodyStr := c.body
		var rels []okf.Relationship
		for _, link := range c.links {
			rels = append(rels, okf.Relationship{
				Label:  "references",
				Target: "/" + link + ".md",
				Text:   refName(link),
			})
		}
		bodyStr = okf.AppendRelationshipsSection(bodyStr, "Relationships", rels)

		description := meta[c.rel]
		if description == "" {
			description = placeholderFor(c)
		}
		tagKind := strings.ToLower(strings.TrimPrefix(c.typ, "GraphQL "))
		fresh := okf.ConceptDoc{
			Frontmatter: okf.Frontmatter{
				Type:        c.typ,
				Title:       c.title,
				Description: description,
				Resource:    "graphql:///" + filepath.ToSlash(schemaBase) + "#/" + c.rel,
				Tags:        []string{"graphql", tagKind},
				Timestamp:   timestamp,
			},
			Body: bodyStr,
		}

		conceptPath := filepath.Join(*outDir, filepath.FromSlash(c.rel)+".md")
		if err := os.MkdirAll(filepath.Dir(conceptPath), 0o755); err != nil {
			log.Fatalf("Failed to create concept subdirectories: %v", err)
		}
		var existing *okf.ConceptDoc
		if e, err := okf.ReadConceptDoc(conceptPath); err == nil {
			existing = e
		}
		merged, changed := okf.MergeConcept(existing, fresh)
		if !changed {
			fmt.Printf("Unchanged, preserved: %s\n", conceptPath)
			continue
		}
		if err := okf.WriteConceptDoc(conceptPath, merged); err != nil {
			log.Fatalf("Failed to write concept doc for %s: %v", c.rel, err)
		}
		kind, action := "Update", "Structure changed for"
		if existing == nil {
			kind, action = "Creation", "Established"
		}
		if err := okf.AppendLogEntry(*outDir, today, kind, fmt.Sprintf("%s [%s](/%s.md).", action, c.title, c.rel)); err != nil {
			log.Fatalf("Failed to append log entry: %v", err)
		}
		fmt.Printf("Produced concept doc: %s\n", conceptPath)
	}

	// Root index.md (only okf_version frontmatter).
	var indexBody bytes.Buffer
	indexBody.WriteString("# GraphQL Catalog\n\n")
	indexBody.WriteString("This OKF bundle documents a GraphQL schema.\n\n## Concepts\n\n")
	for _, c := range concepts {
		desc := meta[c.rel]
		if desc == "" {
			desc = c.typ
		}
		fmt.Fprintf(&indexBody, "- [%s](%s.md) - %s\n", c.title, c.rel, desc)
	}
	indexDoc := okf.ConceptDoc{
		Frontmatter: okf.Frontmatter{OKFVersion: "0.1"},
		Body:        indexBody.String(),
	}
	if err := okf.WriteConceptDoc(filepath.Join(*outDir, "index.md"), indexDoc); err != nil {
		log.Fatalf("Failed to write index.md: %v", err)
	}
	fmt.Println("Produced index.md successfully.")
}

func runIngest(args []string) {
	fsSet := flag.NewFlagSet("ingest", flag.ExitOnError)
	schemaPath := fsSet.String("schema", "", "Path to the GraphQL SDL file (required)")
	bundleDir := fsSet.String("bundle", "", "OKF bundle path (required)")
	sync := fsSet.Bool("sync", false, "Write descriptions back to .okf-metadata.yaml (optional)")
	fsSet.Parse(args)

	if *schemaPath == "" || *bundleDir == "" {
		fsSet.Usage()
		os.Exit(1)
	}
	absSchema, err := filepath.Abs(*schemaPath)
	if err != nil {
		log.Fatalf("Failed to resolve schema path: %v", err)
	}
	if _, err := os.Stat(*bundleDir); os.IsNotExist(err) {
		log.Fatalf("Bundle directory not found: %s", *bundleDir)
	}
	schema, err := loadSchema(absSchema)
	if err != nil {
		log.Fatalf("Failed to load GraphQL schema: %v", err)
	}
	valid := map[string]bool{}
	for _, c := range extractConcepts(schema) {
		valid[c.rel] = true
	}
	schemaDir := filepath.Dir(absSchema)
	meta, err := okf.ReadFolderMetadata(schemaDir)
	if err != nil {
		log.Fatalf("Failed to read folder metadata: %v", err)
	}
	metadataUpdated := false

	err = filepath.WalkDir(*bundleDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		rel, err := filepath.Rel(*bundleDir, path)
		if err != nil {
			return err
		}
		relSlash := filepath.ToSlash(rel)
		if relSlash == "index.md" || relSlash == "log.md" {
			return nil
		}
		conceptRel := strings.TrimSuffix(relSlash, ".md")
		cdoc, err := okf.ReadConceptDoc(path)
		if err != nil {
			log.Fatalf("Failed to read concept doc %s: %v", path, err)
		}
		if !valid[conceptRel] {
			fmt.Printf("Concept '%s' no longer present in the schema (drift).\n", conceptRel)
			return nil
		}
		okfDesc := strings.TrimSpace(cdoc.Frontmatter.Description)
		if okfDesc != strings.TrimSpace(meta[conceptRel]) {
			fmt.Printf("Concept '%s' description mismatch:\n  OKF: %q\n  Metadata: %q\n", conceptRel, okfDesc, strings.TrimSpace(meta[conceptRel]))
			if *sync {
				meta[conceptRel] = okfDesc
				metadataUpdated = true
			}
		}
		return nil
	})
	if err != nil {
		log.Fatalf("Walking bundle failed: %v", err)
	}

	if metadataUpdated && *sync {
		if err := okf.WriteFolderMetadata(schemaDir, meta); err != nil {
			log.Fatalf("Failed to write folder metadata: %v", err)
		}
		fmt.Println("  -> Successfully updated .okf-metadata.yaml.")
	}
	fmt.Println("OKF bundle ingestion / description sync finished.")
}

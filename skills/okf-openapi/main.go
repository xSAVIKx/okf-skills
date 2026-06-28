// Package main implements the OpenAPI OKF (Open Knowledge Format) connector. It parses
// an OpenAPI 3.x or Swagger 2.0 spec into one concept per operation (API Endpoint) and
// per component schema (Schema), links endpoints to the schemas they reference, and
// syncs descriptions back to a .okf-metadata.yaml sidecar next to the spec.
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
	fmt.Println("Usage: okf-openapi <command> [options]")
	fmt.Println("Commands:")
	fmt.Println("  produce  - Create an OKF bundle from an OpenAPI/Swagger spec")
	fmt.Println("  ingest   - Verify a bundle against the spec; --sync writes descriptions to .okf-metadata.yaml")
}

// placeholderFor returns the deterministic placeholder description for a concept.
func placeholderFor(c concept) string {
	if c.typ == "Schema" {
		return "API schema " + c.title
	}
	return "API endpoint " + c.title
}

func runProduce(args []string) {
	fsSet := flag.NewFlagSet("produce", flag.ExitOnError)
	specPath := fsSet.String("spec", "", "Path to the OpenAPI/Swagger spec file (required)")
	outDir := fsSet.String("out", "", "Output bundle directory (required)")
	fsSet.Parse(args)

	if *specPath == "" || *outDir == "" {
		fsSet.Usage()
		os.Exit(1)
	}
	absSpec, err := filepath.Abs(*specPath)
	if err != nil {
		log.Fatalf("Failed to resolve spec path: %v", err)
	}
	doc, err := loadSpec(absSpec)
	if err != nil {
		log.Fatalf("Failed to load spec: %v", err)
	}
	specDir := filepath.Dir(absSpec)
	meta, err := okf.ReadFolderMetadata(specDir)
	if err != nil {
		log.Fatalf("Failed to read folder metadata: %v", err)
	}

	concepts := extractConcepts(doc)
	timestamp := time.Now().Format(time.RFC3339)
	today := time.Now().Format("2006-01-02")
	specBase := filepath.Base(absSpec)

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
		tagKind := "endpoint"
		if c.typ == "Schema" {
			tagKind = "schema"
		}
		fresh := okf.ConceptDoc{
			Frontmatter: okf.Frontmatter{
				Type:        c.typ,
				Title:       c.title,
				Description: description,
				Resource:    "openapi:///" + filepath.ToSlash(specBase) + "#/" + c.rel,
				Tags:        []string{"openapi", tagKind},
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
	title := "API"
	if doc.Info != nil && doc.Info.Title != "" {
		title = doc.Info.Title
	}
	fmt.Fprintf(&indexBody, "# API Catalog: %s\n\n", title)
	indexBody.WriteString("This OKF bundle documents an OpenAPI specification.\n\n## Concepts\n\n")
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
	specPath := fsSet.String("spec", "", "Path to the OpenAPI/Swagger spec file (required)")
	bundleDir := fsSet.String("bundle", "", "OKF bundle path (required)")
	sync := fsSet.Bool("sync", false, "Write descriptions back to .okf-metadata.yaml (optional)")
	fsSet.Parse(args)

	if *specPath == "" || *bundleDir == "" {
		fsSet.Usage()
		os.Exit(1)
	}
	absSpec, err := filepath.Abs(*specPath)
	if err != nil {
		log.Fatalf("Failed to resolve spec path: %v", err)
	}
	if _, err := os.Stat(*bundleDir); os.IsNotExist(err) {
		log.Fatalf("Bundle directory not found: %s", *bundleDir)
	}
	doc, err := loadSpec(absSpec)
	if err != nil {
		log.Fatalf("Failed to load spec: %v", err)
	}
	valid := map[string]bool{}
	for _, c := range extractConcepts(doc) {
		valid[c.rel] = true
	}
	specDir := filepath.Dir(absSpec)
	meta, err := okf.ReadFolderMetadata(specDir)
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
			fmt.Printf("Concept '%s' no longer present in the spec (drift).\n", conceptRel)
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
		if err := okf.WriteFolderMetadata(specDir, meta); err != nil {
			log.Fatalf("Failed to write folder metadata: %v", err)
		}
		fmt.Println("  -> Successfully updated .okf-metadata.yaml.")
	}
	fmt.Println("OKF bundle ingestion / description sync finished.")
}

// Package main implements the local filesystem OKF (Open Knowledge Format) connector.
// It recursively documents directory structures and files, matching ignore rules,
// and syncs descriptions to a central .okf-metadata.yaml.
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

// main is the CLI entrypoint for filesystem connector.
// version is the build version, injected via -ldflags "-X main.version=..." by
// install.sh; it defaults to "dev" for plain `go build`.
var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
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
		fmt.Printf("Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

// printUsage outputs the available CLI commands.
func printUsage() {
	fmt.Println("Usage: okf-fs <command> [options]")
	fmt.Println("Commands:")
	fmt.Println("  produce  - Create OKF bundle from local directory")
	fmt.Println("  ingest   - Sync OKF bundle descriptions back to local directory metadata")
}

// runProduce implements the 'produce' subcommand, scanning the filesystem,
// filtering ignored paths, and generating OKF Markdown files.
func runProduce(args []string) {
	fsSet := flag.NewFlagSet("produce", flag.ExitOnError)
	dirPath := fsSet.String("dir", "", "Local directory to document (required)")
	outDir := fsSet.String("out", "", "Output bundle directory (required)")
	fsSet.Parse(args)

	if *dirPath == "" || *outDir == "" {
		fsSet.Usage()
		os.Exit(1)
	}

	absDir, err := filepath.Abs(*dirPath)
	if err != nil {
		log.Fatalf("Failed to resolve absolute path of directory: %v", err)
	}

	im, err := okf.NewIgnoreMatcher(absDir)
	if err != nil {
		log.Fatalf("Failed to initialize ignore matcher: %v", err)
	}

	meta, err := okf.ReadFolderMetadata(absDir)
	if err != nil {
		log.Fatalf("Failed to read folder metadata: %v", err)
	}

	var paths []string
	err = filepath.WalkDir(absDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == absDir {
			return nil
		}

		rel, err := filepath.Rel(absDir, path)
		if err != nil {
			return err
		}

		// Check ignore rules
		if im.Matches(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		paths = append(paths, rel)
		return nil
	})
	if err != nil {
		log.Fatalf("Directory traversal failed: %v", err)
	}

	timestamp := time.Now().Format(time.RFC3339)
	today := time.Now().Format("2006-01-02")

	// Create OKF bundle documents
	for _, rel := range paths {
		fullPath := filepath.Join(absDir, rel)
		fi, err := os.Stat(fullPath)
		if err != nil {
			log.Fatalf("Failed to get file info for %s: %v", rel, err)
		}

		var conceptType string
		var body bytes.Buffer

		if fi.IsDir() {
			conceptType = "Directory"
			body.WriteString(fmt.Sprintf("# Directory: %s\n\n", filepath.Base(rel)))
			body.WriteString("This directory contains files and folders documented in the OKF bundle.\n")
		} else {
			conceptType = "File"
			body.WriteString(fmt.Sprintf("# File: %s\n\n", filepath.Base(rel)))
			body.WriteString("## Metadata\n\n")
			body.WriteString(fmt.Sprintf("- **Size**: %d bytes\n", fi.Size()))
			body.WriteString(fmt.Sprintf("- **Last Modified**: %s\n", fi.ModTime().Format(time.RFC3339)))
		}

		description := meta[filepath.ToSlash(rel)]
		if description == "" {
			if fi.IsDir() {
				description = fmt.Sprintf("Directory %s", filepath.Base(rel))
			} else {
				description = fmt.Sprintf("File %s", filepath.Base(rel))
			}
		}

		fresh := okf.ConceptDoc{
			Frontmatter: okf.Frontmatter{
				Type:        conceptType,
				Title:       filepath.Base(rel),
				Description: description,
				Resource:    fmt.Sprintf("file:///%s", filepath.ToSlash(filepath.Join(absDir, rel))),
				Tags:        []string{"fs", strings.ToLower(conceptType)},
				Timestamp:   timestamp,
			},
			Body: body.String(),
		}

		// Determine output file path
		conceptPath := filepath.Join(*outDir, rel+".md")
		if err := os.MkdirAll(filepath.Dir(conceptPath), 0755); err != nil {
			log.Fatalf("Failed to create concept subdirectories: %v", err)
		}

		// Incremental produce: preserve an unchanged concept byte-for-byte (keeping
		// any enriched description/body), rewrite only when the structure changed.
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
			log.Fatalf("Failed to write concept doc for %s: %v", rel, err)
		}
		kind, action := "Update", "Structure changed for"
		if existing == nil {
			kind, action = "Creation", "Established"
		}
		bundlePath := "/" + filepath.ToSlash(rel) + ".md"
		if err := okf.AppendLogEntry(*outDir, today, kind, fmt.Sprintf("%s [%s](%s).", action, filepath.Base(rel), bundlePath)); err != nil {
			log.Fatalf("Failed to append log entry: %v", err)
		}
		fmt.Printf("Produced concept doc: %s\n", conceptPath)
	}

	// Generate root index.md
	var indexBody bytes.Buffer
	fmt.Fprintf(&indexBody, "# Directory Catalog: %s\n\n", filepath.Base(absDir))
	indexBody.WriteString("This OKF bundle represents the local filesystem tree structure.\n\n")
	indexBody.WriteString("## Assets\n\n")
	for _, rel := range paths {
		desc := meta[filepath.ToSlash(rel)]
		if desc == "" {
			desc = "Local asset"
		}
		fmt.Fprintf(&indexBody, "- [%s](%s.md) - %s\n", rel, filepath.ToSlash(rel), desc)
	}

	indexDoc := okf.ConceptDoc{
		Frontmatter: okf.Frontmatter{
			OKFVersion: "0.1",
		},
		Body: indexBody.String(),
	}

	if err := okf.WriteConceptDoc(filepath.Join(*outDir, "index.md"), indexDoc); err != nil {
		log.Fatalf("Failed to write index.md: %v", err)
	}
	fmt.Println("Produced index.md successfully.")
}

// runIngest implements the 'ingest' subcommand, validating bundle files and
// optionally updating the target directory's .okf-metadata.yaml description store.
func runIngest(args []string) {
	fsSet := flag.NewFlagSet("ingest", flag.ExitOnError)
	dirPath := fsSet.String("dir", "", "Local directory (required)")
	bundleDir := fsSet.String("bundle", "", "OKF bundle path (required)")
	sync := fsSet.Bool("sync", false, "Write descriptions back to .okf-metadata.yaml (optional)")
	fsSet.Parse(args)

	if *dirPath == "" || *bundleDir == "" {
		fsSet.Usage()
		os.Exit(1)
	}

	absDir, err := filepath.Abs(*dirPath)
	if err != nil {
		log.Fatalf("Failed to resolve absolute path of directory: %v", err)
	}

	if _, err := os.Stat(*bundleDir); os.IsNotExist(err) {
		log.Fatalf("Bundle directory not found: %s", *bundleDir)
	}

	meta, err := okf.ReadFolderMetadata(absDir)
	if err != nil {
		log.Fatalf("Failed to read folder metadata: %v", err)
	}

	metadataUpdated := false

	// Walk OKF bundle to find all concept docs
	err = filepath.WalkDir(*bundleDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == *bundleDir || d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(*bundleDir, path)
		if err != nil {
			return err
		}

		// Skip reserved index/log files
		if rel == "index.md" || rel == "log.md" {
			return nil
		}

		if !strings.HasSuffix(rel, ".md") {
			return nil
		}

		doc, err := okf.ReadConceptDoc(path)
		if err != nil {
			log.Fatalf("Failed to read concept doc %s: %v", path, err)
		}

		// Reconstruct original asset relative path
		assetRel := strings.TrimSuffix(rel, ".md")
		fullAssetPath := filepath.Join(absDir, assetRel)

		// Check if file exists in target directory
		if _, err := os.Stat(fullAssetPath); os.IsNotExist(err) {
			fmt.Printf("Asset '%s' does not exist in target directory.\n", assetRel)
			return nil
		}

		// Sync descriptions
		okfDesc := strings.TrimSpace(doc.Frontmatter.Description)
		dbDesc := strings.TrimSpace(meta[filepath.ToSlash(assetRel)])

		if okfDesc != dbDesc {
			fmt.Printf("Asset '%s' description mismatch:\n  OKF: %q\n  Metadata: %q\n", assetRel, okfDesc, dbDesc)
			if *sync {
				meta[filepath.ToSlash(assetRel)] = okfDesc
				metadataUpdated = true
			}
		}

		return nil
	})
	if err != nil {
		log.Fatalf("Walking bundle failed: %v", err)
	}

	if metadataUpdated && *sync {
		if err := okf.WriteFolderMetadata(absDir, meta); err != nil {
			log.Fatalf("Failed to write folder metadata: %v", err)
		}
		fmt.Println("  -> Successfully updated .okf-metadata.yaml.")
	}

	fmt.Println("OKF bundle ingestion / description sync finished.")
}

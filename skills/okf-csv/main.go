// Package main implements the CSV OKF (Open Knowledge Format) connector. It walks a
// directory of CSV files (honoring .okfignore), documents each file as a concept with
// an inferred column schema and optional data profile, and syncs descriptions back to
// a central .okf-metadata.yaml.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

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
	fmt.Println("Usage: okf-csv <command> [options]")
	fmt.Println("Commands:")
	fmt.Println("  produce  - Create an OKF bundle from a directory of CSV files")
	fmt.Println("  ingest   - Sync OKF bundle descriptions back to .okf-metadata.yaml")
}

// csvConceptPath maps a CSV's path relative to the source dir to its concept path
// relative to the bundle root: "sub/orders.csv" -> "tables/sub/orders.md".
func csvConceptPath(assetRel string) string {
	return "tables/" + strings.TrimSuffix(filepath.ToSlash(assetRel), ".csv") + ".md"
}

// csvAssetPath is the inverse of csvConceptPath: "tables/sub/orders.md" -> "sub/orders.csv".
func csvAssetPath(conceptRel string) string {
	return strings.TrimSuffix(strings.TrimPrefix(filepath.ToSlash(conceptRel), "tables/"), ".md") + ".csv"
}

// runProduce scans a directory for CSV files and emits one concept per file.
func runProduce(args []string) {
	fsSet := flag.NewFlagSet("produce", flag.ExitOnError)
	dirPath := fsSet.String("dir", "", "Directory of CSV files to document (required)")
	outDir := fsSet.String("out", "", "Output bundle directory (required)")
	sample := fsSet.Int("sample", 0, "Sample rows to embed per file (0 = none)")
	profile := fsSet.Bool("profile", false, "Embed a per-column Data Profile section")
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

	// Collect CSV files (sorted for deterministic output).
	var csvFiles []string
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
		if im.Matches(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.IsDir() && strings.EqualFold(filepath.Ext(rel), ".csv") {
			csvFiles = append(csvFiles, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		log.Fatalf("Directory traversal failed: %v", err)
	}
	sort.Strings(csvFiles)

	timestamp := time.Now().Format(time.RFC3339)
	today := time.Now().Format("2006-01-02")

	for _, assetRel := range csvFiles {
		fullPath := filepath.Join(absDir, filepath.FromSlash(assetRel))
		header, rows, err := readCSV(fullPath)
		if err != nil {
			log.Fatalf("Failed to read CSV %s: %v", assetRel, err)
		}
		name := strings.TrimSuffix(filepath.Base(assetRel), filepath.Ext(assetRel))
		types := columnTypes(header, rows)

		bodyStr := renderColumns(header, types)
		if *profile && len(header) > 0 {
			bodyStr = okf.UpsertSection(bodyStr, "Data Profile", okf.RenderProfileSection(columnProfiles(header, rows, types)))
		}
		if *sample > 0 && len(rows) > 0 {
			n := *sample
			if n > len(rows) {
				n = len(rows)
			}
			bodyStr = okf.UpsertSection(bodyStr, "Sample", okf.RenderSampleSection(header, rows[:n]))
		}

		description := meta[assetRel]
		if description == "" {
			description = fmt.Sprintf("CSV file %s", name)
		}
		fresh := okf.ConceptDoc{
			Frontmatter: okf.Frontmatter{
				Type:        "CSV File",
				Title:       name,
				Description: description,
				Resource:    "file:///" + filepath.ToSlash(filepath.Join(absDir, filepath.FromSlash(assetRel))),
				Tags:        []string{"csv", "table"},
				Timestamp:   timestamp,
			},
			Body: bodyStr,
		}

		conceptRel := csvConceptPath(assetRel)
		conceptPath := filepath.Join(*outDir, filepath.FromSlash(conceptRel))
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
			log.Fatalf("Failed to write concept doc for %s: %v", assetRel, err)
		}
		kind, action := "Update", "Structure changed for"
		if existing == nil {
			kind, action = "Creation", "Established"
		}
		if err := okf.AppendLogEntry(*outDir, today, kind, fmt.Sprintf("%s [%s](/%s).", action, name, conceptRel)); err != nil {
			log.Fatalf("Failed to append log entry: %v", err)
		}
		fmt.Printf("Produced concept doc: %s\n", conceptPath)
	}

	// Root index.md (only okf_version frontmatter).
	var indexBody bytes.Buffer
	fmt.Fprintf(&indexBody, "# CSV Catalog: %s\n\n", filepath.Base(absDir))
	indexBody.WriteString("This OKF bundle documents a directory of CSV files.\n\n## Files\n\n")
	for _, assetRel := range csvFiles {
		conceptRel := csvConceptPath(assetRel)
		desc := meta[assetRel]
		if desc == "" {
			desc = "CSV file"
		}
		fmt.Fprintf(&indexBody, "- [%s](%s) - %s\n", assetRel, conceptRel, desc)
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

// runIngest verifies each bundle concept against its CSV file and, with --sync,
// writes enriched descriptions back to the directory's .okf-metadata.yaml.
func runIngest(args []string) {
	fsSet := flag.NewFlagSet("ingest", flag.ExitOnError)
	dirPath := fsSet.String("dir", "", "Directory of CSV files (required)")
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
		if relSlash == "index.md" || relSlash == "log.md" || !strings.HasPrefix(relSlash, "tables/") {
			return nil
		}
		doc, err := okf.ReadConceptDoc(path)
		if err != nil {
			log.Fatalf("Failed to read concept doc %s: %v", path, err)
		}
		assetRel := csvAssetPath(relSlash)
		fullAsset := filepath.Join(absDir, filepath.FromSlash(assetRel))
		if _, err := os.Stat(fullAsset); os.IsNotExist(err) {
			fmt.Printf("CSV '%s' does not exist in target directory.\n", assetRel)
			return nil
		}
		// Verify the bundle's column names still match the CSV header.
		if header, _, err := readCSV(fullAsset); err == nil {
			if want := strings.Join(header, ","); !columnsMatch(doc.Body, header) {
				fmt.Printf("CSV '%s' header drift: file columns = [%s]\n", assetRel, want)
			}
		}
		// Sync descriptions.
		okfDesc := strings.TrimSpace(doc.Frontmatter.Description)
		if okfDesc != strings.TrimSpace(meta[assetRel]) {
			fmt.Printf("CSV '%s' description mismatch:\n  OKF: %q\n  Metadata: %q\n", assetRel, okfDesc, strings.TrimSpace(meta[assetRel]))
			if *sync {
				meta[assetRel] = okfDesc
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

// columnsMatch reports whether the concept's "# Columns" table names match the CSV
// header in order.
func columnsMatch(body string, header []string) bool {
	section, ok := okf.GetSectionAny(body, "Columns")
	if !ok {
		return false
	}
	var names []string
	seenHeader := false
	for _, ln := range strings.Split(section, "\n") {
		ln = strings.TrimSpace(ln)
		if !strings.HasPrefix(ln, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(ln, "|"), "|")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		if !seenHeader { // the "Name | Type" header row
			seenHeader = true
			continue
		}
		if len(cells) > 0 && strings.HasPrefix(cells[0], "---") {
			continue
		}
		if len(cells) > 0 && cells[0] != "" {
			names = append(names, cells[0])
		}
	}
	if len(names) != len(header) {
		return false
	}
	for i := range names {
		if names[i] != strings.TrimSpace(header[i]) {
			return false
		}
	}
	return true
}

// Package main implements the Git OKF (Open Knowledge Format) connector.
// It retrieves tracked file/directory structures and queries commit history
// using go-git, generating OKF bundles, and syncing back to .okf-metadata.yaml.
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

	"github.com/go-git/go-git/v5"
	"github.com/xSAVIKx/okf-skills/okf-go"
)

// main is the CLI entrypoint for git connector.
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
	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

// printUsage outputs the available CLI commands.
func printUsage() {
	fmt.Println("Usage: okf-git <command> [options]")
	fmt.Println("Commands:")
	fmt.Println("  produce  - Create OKF bundle from local Git repository")
	fmt.Println("  ingest   - Sync OKF bundle descriptions back to local Git repository metadata")
}

// gitIgnoreMatcher manages ignore matching using combined git/okf rules.
type gitIgnoreMatcher struct {
	patterns []string
}

func newGitIgnoreMatcher(repoPath string) *gitIgnoreMatcher {
	var patterns []string
	// Load .gitignore
	if data, err := os.ReadFile(filepath.Join(repoPath, ".gitignore")); err == nil {
		patterns = append(patterns, parseIgnorePatterns(string(data))...)
	}
	// Load .okfignore
	if data, err := os.ReadFile(filepath.Join(repoPath, ".okfignore")); err == nil {
		patterns = append(patterns, parseIgnorePatterns(string(data))...)
	}
	return &gitIgnoreMatcher{patterns: patterns}
}

func parseIgnorePatterns(content string) []string {
	var patterns []string
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, filepath.ToSlash(line))
	}
	return patterns
}

func (im *gitIgnoreMatcher) Matches(relPath string) bool {
	relPath = filepath.ToSlash(relPath)
	if relPath == ".git" || strings.HasPrefix(relPath, ".git/") || relPath == ".gitignore" || relPath == ".okfignore" {
		return true
	}

	for _, pattern := range im.patterns {
		cleanPattern := strings.Trim(pattern, "/")
		matched, _ := filepath.Match(cleanPattern, relPath)
		if matched {
			return true
		}
		if strings.HasPrefix(relPath, cleanPattern+"/") {
			return true
		}
		if strings.HasPrefix(cleanPattern, "*.") {
			if strings.HasSuffix(relPath, cleanPattern[1:]) || strings.Contains(relPath, cleanPattern[1:]+"/") {
				return true
			}
		}
	}
	return false
}

// runProduce implements the 'produce' subcommand, scanning git repository files
// and extracting commit provenance.
func runProduce(args []string) {
	fsSet := flag.NewFlagSet("produce", flag.ExitOnError)
	repoPath := fsSet.String("repo", "", "Git repository path (required)")
	outDir := fsSet.String("out", "", "Output bundle directory (required)")
	relationships := fsSet.Bool("relationships", false, "Extract file co-change edges into a Related Files section")
	cochangeMin := fsSet.Int("cochange-min", 2, "Minimum co-change count for a Related Files edge")
	cochangeTop := fsSet.Int("cochange-top", 10, "Maximum Related Files partners per file (0 = unlimited)")
	fsSet.Parse(args)

	if *repoPath == "" || *outDir == "" {
		fsSet.Usage()
		os.Exit(1)
	}

	absRepo, err := filepath.Abs(*repoPath)
	if err != nil {
		log.Fatalf("Failed to resolve absolute path of repo: %v", err)
	}

	repo, err := git.PlainOpen(absRepo)
	if err != nil {
		log.Fatalf("Failed to open git repository: %v", err)
	}

	im := newGitIgnoreMatcher(absRepo)

	meta, err := okf.ReadFolderMetadata(absRepo)
	if err != nil {
		log.Fatalf("Failed to read folder metadata: %v", err)
	}

	var paths []string
	err = filepath.WalkDir(absRepo, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == absRepo {
			return nil
		}

		rel, err := filepath.Rel(absRepo, path)
		if err != nil {
			return err
		}

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
		log.Fatalf("Filesystem walk failed: %v", err)
	}

	// Optionally compute file co-change edges once over the whole history.
	var coIdx *coChangeIndex
	existsConcept := func(string) bool { return true }
	if *relationships {
		coIdx, err = buildCoChangeIndex(repo, im)
		if err != nil {
			log.Fatalf("Failed to compute co-change index: %v", err)
		}
		pathSet := make(map[string]bool, len(paths))
		for _, p := range paths {
			pathSet[filepath.ToSlash(p)] = true
		}
		existsConcept = func(c string) bool { return pathSet[c] }
	}

	today := time.Now().Format("2006-01-02")

	for _, rel := range paths {
		fullPath := filepath.Join(absRepo, rel)
		fi, err := os.Stat(fullPath)
		if err != nil {
			log.Fatalf("Failed to stat file %s: %v", rel, err)
		}

		var conceptType string
		var body bytes.Buffer

		// Query commit log for file history
		var lastAuthor string
		var lastCommitTime time.Time
		var lastCommitMsg string

		// Copy the loop variable: git.LogOptions retains the *string, and closing
		// the iterator on each iteration (rather than deferring to function
		// return) avoids leaking one open commit iterator per file on a large repo.
		name := rel
		cIter, err := repo.Log(&git.LogOptions{FileName: &name})
		if err == nil {
			if commit, err := cIter.Next(); err == nil {
				lastAuthor = commit.Author.Name
				lastCommitTime = commit.Author.When
				lastCommitMsg = strings.TrimSpace(commit.Message)
			}
			cIter.Close()
		}

		if lastCommitTime.IsZero() {
			lastCommitTime = fi.ModTime()
		}

		if fi.IsDir() {
			conceptType = "Git Directory"
			body.WriteString(fmt.Sprintf("# Git Directory: %s\n\n", filepath.Base(rel)))
			body.WriteString("This directory is part of the Git repository file tree.\n\n")
		} else {
			conceptType = "Git File"
			body.WriteString(fmt.Sprintf("# Git File: %s\n\n", filepath.Base(rel)))
			body.WriteString("## Metadata\n\n")
			body.WriteString(fmt.Sprintf("- **Size**: %d bytes\n", fi.Size()))
			if lastAuthor != "" {
				body.WriteString(fmt.Sprintf("- **Last Committer**: %s\n", lastAuthor))
				body.WriteString(fmt.Sprintf("- **Last Commit Date**: %s\n", lastCommitTime.Format(time.RFC3339)))
				body.WriteString(fmt.Sprintf("- **Last Commit Message**: %q\n", lastCommitMsg))
			} else {
				body.WriteString(fmt.Sprintf("- **Last Modified**: %s\n", fi.ModTime().Format(time.RFC3339)))
			}
		}

		description := meta[filepath.ToSlash(rel)]
		if description == "" {
			if fi.IsDir() {
				description = fmt.Sprintf("Git directory %s", filepath.Base(rel))
			} else {
				description = fmt.Sprintf("Git file %s", filepath.Base(rel))
			}
		}

		bodyStr := body.String()
		if *relationships && !fi.IsDir() {
			rels := coIdx.relationshipsFor(filepath.ToSlash(rel), *cochangeMin, *cochangeTop, existsConcept)
			bodyStr = okf.AppendRelationshipsSection(bodyStr, "Related Files", rels)
		}

		fresh := okf.ConceptDoc{
			Frontmatter: okf.Frontmatter{
				Type:        conceptType,
				Title:       filepath.Base(rel),
				Description: description,
				Resource:    fmt.Sprintf("git:///%s", filepath.ToSlash(filepath.Join(absRepo, rel))),
				Tags:        []string{"git", strings.ToLower(strings.Fields(conceptType)[1])},
				Timestamp:   lastCommitTime.Format(time.RFC3339),
			},
			Body: bodyStr,
		}

		conceptPath := filepath.Join(*outDir, rel+".md")
		if err := os.MkdirAll(filepath.Dir(conceptPath), 0755); err != nil {
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
			log.Fatalf("Failed to write concept doc: %v", err)
		}
		kind, action := "Update", "Structure changed for"
		if existing == nil {
			kind, action = "Creation", "Established"
		}
		bundlePath := "/" + filepath.ToSlash(rel) + ".md"
		_ = okf.AppendLogEntry(*outDir, today, kind, fmt.Sprintf("%s [%s](%s).", action, filepath.Base(rel), bundlePath))
		fmt.Printf("Produced concept doc: %s\n", conceptPath)
	}

	// Produce bundle-root index.md
	var indexBody bytes.Buffer
	fmt.Fprintf(&indexBody, "# Git Repository: %s\n\n", filepath.Base(absRepo))
	indexBody.WriteString("This OKF bundle represents the tracked files and histories in the Git repository.\n\n")
	indexBody.WriteString("## Assets\n\n")
	for _, rel := range paths {
		desc := meta[filepath.ToSlash(rel)]
		if desc == "" {
			desc = "Git repository asset"
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

// runIngest implements the 'ingest' subcommand, validating bundle files
// and updating the git repository's .okf-metadata.yaml file.
func runIngest(args []string) {
	fsSet := flag.NewFlagSet("ingest", flag.ExitOnError)
	repoPath := fsSet.String("repo", "", "Git repository path (required)")
	bundleDir := fsSet.String("bundle", "", "OKF bundle path (required)")
	sync := fsSet.Bool("sync", false, "Write descriptions back to .okf-metadata.yaml (optional)")
	fsSet.Parse(args)

	if *repoPath == "" || *bundleDir == "" {
		fsSet.Usage()
		os.Exit(1)
	}

	absRepo, err := filepath.Abs(*repoPath)
	if err != nil {
		log.Fatalf("Failed to resolve absolute path of repo: %v", err)
	}

	if _, err := os.Stat(*bundleDir); os.IsNotExist(err) {
		log.Fatalf("Bundle directory not found: %s", *bundleDir)
	}

	meta, err := okf.ReadFolderMetadata(absRepo)
	if err != nil {
		log.Fatalf("Failed to read folder metadata: %v", err)
	}

	metadataUpdated := false

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

		assetRel := strings.TrimSuffix(rel, ".md")
		fullAssetPath := filepath.Join(absRepo, assetRel)

		if _, err := os.Stat(fullAssetPath); os.IsNotExist(err) {
			fmt.Printf("Asset '%s' does not exist in target Git repository.\n", assetRel)
			return nil
		}

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
		if err := okf.WriteFolderMetadata(absRepo, meta); err != nil {
			log.Fatalf("Failed to write folder metadata: %v", err)
		}
		fmt.Println("  -> Successfully updated .okf-metadata.yaml.")
	}

	fmt.Println("OKF bundle ingestion / description sync finished.")
}

package okf

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestRoundTrip verifies that a concept doc can be written to disk and parsed back exactly.
func TestRoundTrip(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "okf-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	filePath := filepath.Join(tempDir, "concept.md")
	originalDoc := ConceptDoc{
		Frontmatter: Frontmatter{
			Type:        "Test Type",
			Title:       "Test Title",
			Description: "Test Description",
			Resource:    "test://resource/path",
			Tags:        []string{"tag1", "tag2"},
			Timestamp:   "2026-06-13T11:00:00Z",
		},
		Body: "# Header\n\nSome body text here.",
	}

	if err := WriteConceptDoc(filePath, originalDoc); err != nil {
		t.Fatalf("failed to write concept doc: %v", err)
	}

	parsedDoc, err := ReadConceptDoc(filePath)
	if err != nil {
		t.Fatalf("failed to read concept doc: %v", err)
	}

	if parsedDoc.Frontmatter.Type != originalDoc.Frontmatter.Type {
		t.Errorf("expected Type %q, got %q", originalDoc.Frontmatter.Type, parsedDoc.Frontmatter.Type)
	}
	if parsedDoc.Frontmatter.Title != originalDoc.Frontmatter.Title {
		t.Errorf("expected Title %q, got %q", originalDoc.Frontmatter.Title, parsedDoc.Frontmatter.Title)
	}
	if parsedDoc.Frontmatter.Description != originalDoc.Frontmatter.Description {
		t.Errorf("expected Description %q, got %q", originalDoc.Frontmatter.Description, parsedDoc.Frontmatter.Description)
	}
	if parsedDoc.Frontmatter.Resource != originalDoc.Frontmatter.Resource {
		t.Errorf("expected Resource %q, got %q", originalDoc.Frontmatter.Resource, parsedDoc.Frontmatter.Resource)
	}
	if len(parsedDoc.Frontmatter.Tags) != len(originalDoc.Frontmatter.Tags) || parsedDoc.Frontmatter.Tags[0] != originalDoc.Frontmatter.Tags[0] {
		t.Errorf("expected Tags %v, got %v", originalDoc.Frontmatter.Tags, parsedDoc.Frontmatter.Tags)
	}
	if parsedDoc.Frontmatter.Timestamp != originalDoc.Frontmatter.Timestamp {
		t.Errorf("expected Timestamp %q, got %q", originalDoc.Frontmatter.Timestamp, parsedDoc.Frontmatter.Timestamp)
	}
	if strings.TrimSpace(parsedDoc.Body) != strings.TrimSpace(originalDoc.Body) {
		t.Errorf("expected Body %q, got %q", originalDoc.Body, parsedDoc.Body)
	}
}

// TestLineEndings checks that both LF and CRLF delimited frontmatter blocks are correctly parsed.
func TestLineEndings(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "okf-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	lfContent := "---\ntype: Test\ntitle: LF Doc\n---\nBody with LF"
	crlfContent := "---\r\ntype: Test\r\ntitle: CRLF Doc\r\n---\r\nBody with CRLF"

	lfPath := filepath.Join(tempDir, "lf.md")
	crlfPath := filepath.Join(tempDir, "crlf.md")

	if err := os.WriteFile(lfPath, []byte(lfContent), 0644); err != nil {
		t.Fatalf("failed to write LF test file: %v", err)
	}
	if err := os.WriteFile(crlfPath, []byte(crlfContent), 0644); err != nil {
		t.Fatalf("failed to write CRLF test file: %v", err)
	}

	docLF, err := ReadConceptDoc(lfPath)
	if err != nil {
		t.Fatalf("failed to read LF doc: %v", err)
	}
	if docLF.Frontmatter.Title != "LF Doc" || strings.TrimSpace(docLF.Body) != "Body with LF" {
		t.Errorf("LF doc parsed incorrectly: %+v", docLF)
	}

	docCRLF, err := ReadConceptDoc(crlfPath)
	if err != nil {
		t.Fatalf("failed to read CRLF doc: %v", err)
	}
	if docCRLF.Frontmatter.Title != "CRLF Doc" || strings.TrimSpace(docCRLF.Body) != "Body with CRLF" {
		t.Errorf("CRLF doc parsed incorrectly: %+v", docCRLF)
	}
}

// TestIndexFileFrontmatter verifies index files with only okf_version compile and round-trip successfully.
func TestIndexFileFrontmatter(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "okf-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	filePath := filepath.Join(tempDir, "index.md")
	indexDoc := ConceptDoc{
		Frontmatter: Frontmatter{
			OKFVersion: "0.1",
		},
		Body: "# Index\n\n- [Link](item.md)",
	}

	if err := WriteConceptDoc(filePath, indexDoc); err != nil {
		t.Fatalf("failed to write index doc: %v", err)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read index raw file: %v", err)
	}
	rawStr := string(content)
	if !strings.Contains(rawStr, "okf_version: \"0.1\"") {
		t.Errorf("serialized file does not contain okf_version: %s", rawStr)
	}
	if strings.Contains(rawStr, "type:") {
		t.Errorf("serialized file should not contain type if omitted: %s", rawStr)
	}

	parsedDoc, err := ReadConceptDoc(filePath)
	if err != nil {
		t.Fatalf("failed to parse index doc: %v", err)
	}
	if parsedDoc.Frontmatter.OKFVersion != "0.1" {
		t.Errorf("expected OKFVersion '0.1', got %q", parsedDoc.Frontmatter.OKFVersion)
	}
	if parsedDoc.Frontmatter.Type != "" {
		t.Errorf("expected empty Type, got %q", parsedDoc.Frontmatter.Type)
	}
}

// TestMalformedFiles verifies that parsing fails with error when boundary markers are missing.
func TestMalformedFiles(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "okf-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	badPath := filepath.Join(tempDir, "bad.md")
	badContent := "This file does not have frontmatter markers at all"

	if err := os.WriteFile(badPath, []byte(badContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err = ReadConceptDoc(badPath)
	if err == nil {
		t.Error("expected error for malformed file, but got nil")
	}
}

// TestIgnoreMatcher verifies that it parses .okfignore patterns and matches correctly.
func TestIgnoreMatcher(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "okf-ignore-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ignoreContent := `# Comment
*.log
/temp/
build/
`
	if err := os.WriteFile(filepath.Join(tempDir, ".okfignore"), []byte(ignoreContent), 0644); err != nil {
		t.Fatalf("failed to write .okfignore: %v", err)
	}

	im, err := NewIgnoreMatcher(tempDir)
	if err != nil {
		t.Fatalf("failed to create ignore matcher: %v", err)
	}

	tests := []struct {
		path   string
		ignore bool
	}{
		{"test.log", true},
		{"src/main.go", false},
		{".git", true},
		{".git/config", true},
		{".okfignore", true},
		{"temp/file.txt", true},
		{"build/artifact.exe", true},
		{"build/sub/file.json", true},
	}

	for _, test := range tests {
		actual := im.Matches(test.path)
		if actual != test.ignore {
			t.Errorf("Matches(%q): expected %t, got %t", test.path, test.ignore, actual)
		}
	}
}

// TestFolderMetadata verifies loading and saving folder metadata.
func TestFolderMetadata(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "okf-meta-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	meta, err := ReadFolderMetadata(tempDir)
	if err != nil {
		t.Fatalf("failed to read non-existent metadata: %v", err)
	}
	if len(meta) != 0 {
		t.Errorf("expected empty metadata, got %+v", meta)
	}

	originalMeta := map[string]string{
		"README.md":   "Project documentation",
		"src/main.go": "Entry point code",
	}

	if err := WriteFolderMetadata(tempDir, originalMeta); err != nil {
		t.Fatalf("failed to write folder metadata: %v", err)
	}

	parsedMeta, err := ReadFolderMetadata(tempDir)
	if err != nil {
		t.Fatalf("failed to read saved metadata: %v", err)
	}

	if !reflect.DeepEqual(parsedMeta, originalMeta) {
		t.Errorf("expected %+v, got %+v", originalMeta, parsedMeta)
	}
}

// TestOKFSpecV2Frontmatter verifies OKF v0.2 frontmatter parsing, trust tier derivation, and staleness checks.
func TestOKFSpecV2Frontmatter(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "okf-v2-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	yamlContent := `---
type: Attested Computation
title: Revenue Calculation
description: Recognized revenue calculation
runtime: bigquery
status: stable
stale_after: 2026-06-15
generated:
  by: okf-sqlite/v0.2.0
  at: 2026-06-20T14:00:00Z
verified:
  by: human:ahormati
  at: 2026-06-21T09:00:00Z
parameters:
  - name: year
    type: integer
    required: true
executor:
  resource: references/skills/run-bq.md
  receipt: [job_id, executed_sql]
attester:
  resource: references/attesters/sql-check.py
sources:
  - id: rev-policy
    resource: https://wiki.acme/finance/rev-policy
    title: Revenue Policy
    author: team:finance
    usage_count: 500
    last_modified: 2026-05-01
---
# Computation

    SELECT SUM(amount) FROM sales WHERE year = @year
`
	filePath := filepath.Join(tempDir, "computation.md")
	if err := os.WriteFile(filePath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	doc, err := ReadConceptDoc(filePath)
	if err != nil {
		t.Fatalf("failed to read v0.2 concept doc: %v", err)
	}

	fm := doc.Frontmatter
	if fm.Type != "Attested Computation" {
		t.Errorf("expected Type 'Attested Computation', got %q", fm.Type)
	}
	if fm.Runtime != "bigquery" {
		t.Errorf("expected Runtime 'bigquery', got %q", fm.Runtime)
	}
	if fm.Status != "stable" {
		t.Errorf("expected Status 'stable', got %q", fm.Status)
	}
	if fm.Generated == nil || fm.Generated.By != "okf-sqlite/v0.2.0" {
		t.Errorf("expected Generated.By 'okf-sqlite/v0.2.0', got %+v", fm.Generated)
	}
	if fm.GetEffectiveTimestamp() != "2026-06-20T14:00:00Z" {
		t.Errorf("expected GetEffectiveTimestamp '2026-06-20T14:00:00Z', got %q", fm.GetEffectiveTimestamp())
	}

	// Trust tier check
	if fm.GetTrustTier() != TrustTierHumanReviewed {
		t.Errorf("expected TrustTierHumanReviewed, got %q", fm.GetTrustTier())
	}

	// Single verified item unmarshaling check
	if len(fm.Verified) != 1 || fm.Verified[0].By != "human:ahormati" {
		t.Errorf("expected 1 verified item 'human:ahormati', got %+v", fm.Verified)
	}

	// Staleness check
	staleDate, _ := time.Parse("2006-01-02", "2026-06-20")
	freshDate, _ := time.Parse("2006-01-02", "2026-06-10")
	if !fm.IsStale(staleDate) {
		t.Errorf("expected IsStale to be true for date %v with stale_after 2026-06-15", staleDate)
	}
	if fm.IsStale(freshDate) {
		t.Errorf("expected IsStale to be false for date %v with stale_after 2026-06-15", freshDate)
	}

	// Sources check
	if len(fm.Sources) != 1 || fm.Sources[0].ID != "rev-policy" || fm.Sources[0].UsageCount != 500 {
		t.Errorf("expected Sources parsed correctly, got %+v", fm.Sources)
	}

	// Parameters check
	if len(fm.Parameters) != 1 || fm.Parameters[0].Name != "year" || !fm.Parameters[0].Required {
		t.Errorf("expected Parameters parsed correctly, got %+v", fm.Parameters)
	}
}

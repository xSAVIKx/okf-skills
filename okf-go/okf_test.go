package okf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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

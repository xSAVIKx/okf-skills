package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFSIntegration(t *testing.T) {
	binaryPath := getBinaryPath("okf-fs")
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Skipf("FS binary not found at %s. Build it first.", binaryPath)
	}

	tempDir, err := os.MkdirTemp("", "okf-fs-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	srcDir := filepath.Join(tempDir, "my-src")
	bundleDir := filepath.Join(tempDir, "my-bundle")

	if err := os.MkdirAll(filepath.Join(srcDir, "sub"), 0755); err != nil {
		t.Fatalf("failed to create src dir structure: %v", err)
	}

	// Create test files
	if err := os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("hello file1"), 0644); err != nil {
		t.Fatalf("failed to write file1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "sub", "file2.txt"), []byte("hello file2"), 0644); err != nil {
		t.Fatalf("failed to write file2: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "ignored.txt"), []byte("should be ignored"), 0644); err != nil {
		t.Fatalf("failed to write ignored.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, ".okfignore"), []byte("ignored.txt\n"), 0644); err != nil {
		t.Fatalf("failed to write .okfignore: %v", err)
	}

	// Create metadata file
	metaContent := "file1.txt: \"The main configuration file\"\nsub/file2.txt: \"Helper function templates\"\n"
	if err := os.WriteFile(filepath.Join(srcDir, ".okf-metadata.yaml"), []byte(metaContent), 0644); err != nil {
		t.Fatalf("failed to write .okf-metadata.yaml: %v", err)
	}

	// 1. Run produce
	cmdProduce := exec.Command(binaryPath, "produce", "--dir", srcDir, "--out", bundleDir)
	var stderr bytesBuffer
	cmdProduce.Stderr = &stderr
	if err := cmdProduce.Run(); err != nil {
		t.Fatalf("fs produce command failed: %v. Stderr: %s", err, stderr.String())
	}

	// 2. Validate output bundle files
	indexFile := filepath.Join(bundleDir, "index.md")
	file1Doc := filepath.Join(bundleDir, "file1.txt.md")
	file2Doc := filepath.Join(bundleDir, "sub", "file2.txt.md")
	ignoredDoc := filepath.Join(bundleDir, "ignored.txt.md")

	if _, err := os.Stat(indexFile); os.IsNotExist(err) {
		t.Errorf("index.md was not produced")
	}
	if _, err := os.Stat(file1Doc); os.IsNotExist(err) {
		t.Errorf("file1.txt.md was not produced")
	}
	if _, err := os.Stat(file2Doc); os.IsNotExist(err) {
		t.Errorf("sub/file2.txt.md was not produced")
	}
	if _, err := os.Stat(ignoredDoc); !os.IsNotExist(err) {
		t.Errorf("ignored.txt.md should NOT be produced")
	}

	// Check file1Doc description matches metadata
	file1Bytes, _ := os.ReadFile(file1Doc)
	if !strings.Contains(string(file1Bytes), "description: The main configuration file") && !strings.Contains(string(file1Bytes), "description: \"The main configuration file\"") {
		t.Errorf("file1Doc frontmatter description is missing or incorrect: %s", string(file1Bytes))
	}

	// Check index.md contains okf_version: "0.1"
	indexBytes, _ := os.ReadFile(indexFile)
	if !strings.Contains(string(indexBytes), "okf_version: \"0.1\"") {
		t.Errorf("index.md does not contain okf_version: \"0.1\"")
	}

	// 3. Modify description in file1.txt.md and sync back
	modifiedFile1 := strings.Replace(string(file1Bytes), "description: The main configuration file", "description: The main configuration file - updated via OKF", 1)
	modifiedFile1 = strings.Replace(modifiedFile1, "description: \"The main configuration file\"", "description: The main configuration file - updated via OKF", 1)
	if err := os.WriteFile(file1Doc, []byte(modifiedFile1), 0644); err != nil {
		t.Fatalf("failed to write modified file1.txt.md: %v", err)
	}

	stderr.buf = nil
	cmdIngest := exec.Command(binaryPath, "ingest", "--dir", srcDir, "--bundle", bundleDir, "--sync")
	cmdIngest.Stderr = &stderr
	if err := cmdIngest.Run(); err != nil {
		t.Fatalf("fs ingest command failed: %v. Stderr: %s", err, stderr.String())
	}

	// Verify .okf-metadata.yaml was updated
	newMetaBytes, err := os.ReadFile(filepath.Join(srcDir, ".okf-metadata.yaml"))
	if err != nil {
		t.Fatalf("failed to read updated .okf-metadata.yaml: %v", err)
	}
	if !strings.Contains(string(newMetaBytes), "The main configuration file - updated via OKF") {
		t.Errorf(".okf-metadata.yaml was not updated: %s", string(newMetaBytes))
	}
}

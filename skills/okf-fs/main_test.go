package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFSLogic basic check to verify go compiler sees main module as correct.
func TestFSLogic(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "okf-fs-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files
	file1 := filepath.Join(tempDir, "README.md")
	file2 := filepath.Join(tempDir, "data.csv")
	os.WriteFile(file1, []byte("readme content"), 0644)
	os.WriteFile(file2, []byte("data content"), 0644)

	// Verify paths can be resolved and stat matches
	fi1, err := os.Stat(file1)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if fi1.Size() != 14 {
		t.Errorf("expected size 14, got %d", fi1.Size())
	}
	if strings.ToLower(filepath.Base(file1)) != "readme.md" {
		t.Errorf("base name mismatch: %s", filepath.Base(file1))
	}
}

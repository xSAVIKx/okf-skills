package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGitIgnoreMatcher(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "okf-git-ignore-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	gitIgnoreContent := `
# git ignore
target/
*.tmp
`
	okfIgnoreContent := `
# okf ignore
secret.json
`
	if err := os.WriteFile(filepath.Join(tempDir, ".gitignore"), []byte(gitIgnoreContent), 0644); err != nil {
		t.Fatalf("failed to write .gitignore: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, ".okfignore"), []byte(okfIgnoreContent), 0644); err != nil {
		t.Fatalf("failed to write .okfignore: %v", err)
	}

	im := newGitIgnoreMatcher(tempDir)

	tests := []struct {
		path   string
		ignore bool
	}{
		{"target/debug/app.exe", true},
		{"file.tmp", true},
		{"secret.json", true},
		{"src/main.go", false},
		{".git", true},
		{".git/config", true},
		{".gitignore", true},
		{".okfignore", true},
	}

	for _, test := range tests {
		actual := im.Matches(test.path)
		if actual != test.ignore {
			t.Errorf("Matches(%q): expected %t, got %t", test.path, test.ignore, actual)
		}
	}
}

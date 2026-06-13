package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitIntegration(t *testing.T) {
	binaryPath := getBinaryPath("okf-git")
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Skipf("Git binary not found at %s. Build it first.", binaryPath)
	}

	tempDir, err := os.MkdirTemp("", "okf-git-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	repoDir := filepath.Join(tempDir, "my-repo")
	bundleDir := filepath.Join(tempDir, "my-bundle")

	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}

	// Initialize Git repo using os/exec
	runCmd := func(dir string, name string, args ...string) {
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			t.Fatalf("git command failed: %s %v: %v", name, args, err)
		}
	}

	runCmd(repoDir, "git", "init")
	runCmd(repoDir, "git", "config", "user.name", "Test Author")
	runCmd(repoDir, "git", "config", "user.email", "test@example.com")

	// Create a file and commit it
	gitFile := filepath.Join(repoDir, "file_git.txt")
	if err := os.WriteFile(gitFile, []byte("hello git"), 0644); err != nil {
		t.Fatalf("failed to write file_git.txt: %v", err)
	}

	runCmd(repoDir, "git", "add", "file_git.txt")
	runCmd(repoDir, "git", "commit", "-m", "First commit of git file")

	// Create .okf-metadata.yaml
	metaContent := "file_git.txt: \"Git tracked text file\"\n"
	if err := os.WriteFile(filepath.Join(repoDir, ".okf-metadata.yaml"), []byte(metaContent), 0644); err != nil {
		t.Fatalf("failed to write .okf-metadata.yaml: %v", err)
	}

	// 1. Run produce
	cmdProduce := exec.Command(binaryPath, "produce", "--repo", repoDir, "--out", bundleDir)
	var stderr bytesBuffer
	cmdProduce.Stderr = &stderr
	if err := cmdProduce.Run(); err != nil {
		t.Fatalf("git produce command failed: %v. Stderr: %s", err, stderr.String())
	}

	// 2. Validate output bundle files
	indexFile := filepath.Join(bundleDir, "index.md")
	gitDocFile := filepath.Join(bundleDir, "file_git.txt.md")

	if _, err := os.Stat(indexFile); os.IsNotExist(err) {
		t.Errorf("index.md was not produced")
	}
	if _, err := os.Stat(gitDocFile); os.IsNotExist(err) {
		t.Errorf("file_git.txt.md was not produced")
	}

	// Verify index.md contains okf_version: "0.1"
	indexBytes, _ := os.ReadFile(indexFile)
	if !strings.Contains(string(indexBytes), "okf_version: \"0.1\"") {
		t.Errorf("index.md does not contain okf_version: \"0.1\"")
	}

	// Check gitDocFile contains author and commit message provenance
	gitDocBytes, _ := os.ReadFile(gitDocFile)
	gitDocStr := string(gitDocBytes)
	if !strings.Contains(gitDocStr, "Last Committer**: Test Author") {
		t.Errorf("file_git.txt.md does not contain committer: %s", gitDocStr)
	}
	if !strings.Contains(gitDocStr, "First commit of git file") {
		t.Errorf("file_git.txt.md does not contain commit message: %s", gitDocStr)
	}

	// 3. Modify description in file_git.txt.md and sync back
	modifiedGitDoc := strings.Replace(gitDocStr, "description: Git tracked text file", "description: Git tracked text file - modified in bundle", 1)
	modifiedGitDoc = strings.Replace(modifiedGitDoc, "description: \"Git tracked text file\"", "description: Git tracked text file - modified in bundle", 1)
	if err := os.WriteFile(gitDocFile, []byte(modifiedGitDoc), 0644); err != nil {
		t.Fatalf("failed to write modified file_git.txt.md: %v", err)
	}

	stderr.buf = nil
	cmdIngest := exec.Command(binaryPath, "ingest", "--repo", repoDir, "--bundle", bundleDir, "--sync")
	cmdIngest.Stderr = &stderr
	if err := cmdIngest.Run(); err != nil {
		t.Fatalf("git ingest command failed: %v. Stderr: %s", err, stderr.String())
	}

	// Verify .okf-metadata.yaml was updated
	newMetaBytes, err := os.ReadFile(filepath.Join(repoDir, ".okf-metadata.yaml"))
	if err != nil {
		t.Fatalf("failed to read updated .okf-metadata.yaml: %v", err)
	}
	if !strings.Contains(string(newMetaBytes), "Git tracked text file - modified in bundle") {
		t.Errorf(".okf-metadata.yaml was not updated: %s", string(newMetaBytes))
	}
}

// Package okf implements shared types and helpers for managing
// Open Knowledge Format (OKF) concept documents and frontmatter serialization.
package okf

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Frontmatter represents the YAML metadata block at the top of an OKF concept document.
type Frontmatter struct {
	Type        string    `yaml:"type,omitempty"`        // The kind of concept (e.g., SQLite Table, Dataset)
	Title       string    `yaml:"title,omitempty"`       // The display name of the concept
	Description string    `yaml:"description,omitempty"` // A brief summary description
	Resource    string    `yaml:"resource,omitempty"`    // Canonical URI for the underlying asset
	Tags        []string  `yaml:"tags,omitempty"`        // Tags for classification
	Timestamp   string    `yaml:"timestamp,omitempty"`   // ISO 8601 modification timestamp
	OKFVersion  string    `yaml:"okf_version,omitempty"` // OKF version targeted (only permitted in bundle-root index.md)
}

// ConceptDoc represents a complete OKF document, separating YAML frontmatter from the markdown body.
type ConceptDoc struct {
	Frontmatter Frontmatter // YAML metadata
	Body        string      // Markdown documentation body
}

// WriteConceptDoc serializes a ConceptDoc to a file with YAML frontmatter markers.
func WriteConceptDoc(filePath string, doc ConceptDoc) error {
	var buf bytes.Buffer
	buf.WriteString("---\n")
	fmBytes, err := yaml.Marshal(doc.Frontmatter)
	if err != nil {
		return err
	}
	buf.Write(fmBytes)
	buf.WriteString("---\n")
	buf.WriteString(doc.Body)
	return os.WriteFile(filePath, buf.Bytes(), 0644)
}

// ReadConceptDoc parses a markdown file with YAML frontmatter into a ConceptDoc struct.
func ReadConceptDoc(filePath string) (*ConceptDoc, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	parts := bytes.SplitN(content, []byte("---\n"), 3)
	if len(parts) < 3 {
		// Try parsing with CRLF line endings
		parts = bytes.SplitN(content, []byte("---\r\n"), 3)
		if len(parts) < 3 {
			return nil, fmt.Errorf("invalid OKF concept file format: missing frontmatter boundaries")
		}
	}

	var fm Frontmatter
	if err := yaml.Unmarshal(parts[1], &fm); err != nil {
		return nil, err
	}

	return &ConceptDoc{
		Frontmatter: fm,
		Body:        string(parts[2]),
	}, nil
}

// IgnoreMatcher parses .okfignore patterns and checks if paths should be ignored.
type IgnoreMatcher struct {
	patterns []string
}

// NewIgnoreMatcher parses the .okfignore file at the target root.
func NewIgnoreMatcher(root string) (*IgnoreMatcher, error) {
	ignorePath := filepath.Join(root, ".okfignore")
	content, err := os.ReadFile(ignorePath)
	if os.IsNotExist(err) {
		return &IgnoreMatcher{}, nil
	} else if err != nil {
		return nil, err
	}

	var patterns []string
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Convert windows backslashes to slashes
		line = filepath.ToSlash(line)
		patterns = append(patterns, line)
	}
	return &IgnoreMatcher{patterns: patterns}, nil
}

// Matches returns true if the relative path matches any ignore patterns.
func (im *IgnoreMatcher) Matches(relPath string) bool {
	relPath = filepath.ToSlash(relPath)
	// Always ignore .git and okf directories/files (e.g. bundle outputs)
	if relPath == ".git" || strings.HasPrefix(relPath, ".git/") || relPath == ".okfignore" {
		return true
	}

	for _, pattern := range im.patterns {
		// Clean up leading/trailing slashes in pattern
		cleanPattern := strings.Trim(pattern, "/")

		// Standard matches or wildcard matches
		matched, _ := filepath.Match(cleanPattern, relPath)
		if matched {
			return true
		}
		// Match files in a directory if pattern is a directory name
		if strings.HasPrefix(relPath, cleanPattern+"/") {
			return true
		}
		// Match suffix wildcards (e.g. *.log) anywhere in path
		if strings.HasPrefix(cleanPattern, "*.") {
			if strings.HasSuffix(relPath, cleanPattern[1:]) || strings.Contains(relPath, cleanPattern[1:]+"/") {
				return true
			}
		}
	}
	return false
}

// ReadFolderMetadata reads file descriptions from .okf-metadata.yaml in the target directory.
func ReadFolderMetadata(dirPath string) (map[string]string, error) {
	metaPath := filepath.Join(dirPath, ".okf-metadata.yaml")
	content, err := os.ReadFile(metaPath)
	if os.IsNotExist(err) {
		return make(map[string]string), nil
	} else if err != nil {
		return nil, err
	}

	var meta map[string]string
	if err := yaml.Unmarshal(content, &meta); err != nil {
		return nil, err
	}
	return meta, nil
}

// WriteFolderMetadata writes file descriptions to .okf-metadata.yaml in sorted order.
func WriteFolderMetadata(dirPath string, meta map[string]string) error {
	metaPath := filepath.Join(dirPath, ".okf-metadata.yaml")
	if len(meta) == 0 {
		_ = os.Remove(metaPath)
		return nil
	}

	content, err := yaml.Marshal(meta)
	if err != nil {
		return err
	}
	return os.WriteFile(metaPath, content, 0644)
}

// Package okf implements shared types and helpers for managing
// Open Knowledge Format (OKF) concept documents and frontmatter serialization.
package okf

import (
	"bytes"
	"fmt"
	"os"
	"gopkg.in/yaml.v3"
)

// Frontmatter represents the YAML metadata block at the top of an OKF concept document.
type Frontmatter struct {
	Type        string    `yaml:"type"`                  // The kind of concept (e.g., SQLite Table, Dataset)
	Title       string    `yaml:"title,omitempty"`       // The display name of the concept
	Description string    `yaml:"description,omitempty"` // A brief summary description
	Resource    string    `yaml:"resource,omitempty"`    // Canonical URI for the underlying asset
	Tags        []string  `yaml:"tags,omitempty"`        // Tags for classification
	Timestamp   string    `yaml:"timestamp,omitempty"`   // ISO 8601 modification timestamp
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

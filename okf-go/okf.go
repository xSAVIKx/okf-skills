// Package okf implements shared types and helpers for managing
// Open Knowledge Format (OKF) concept documents and frontmatter serialization.
package okf

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// TrustTier represents derived credibility status per OKF-SPEC §5.3.
type TrustTier string

const (
	TrustTierUnverified      TrustTier = "unverified"
	TrustTierMachineConfirmed TrustTier = "machine-confirmed"
	TrustTierHumanReviewed   TrustTier = "human-reviewed"
)

// GeneratedInfo records how the concept was produced per OKF-SPEC §5.2.
type GeneratedInfo struct {
	By string `yaml:"by" json:"by"`
	At string `yaml:"at,omitempty" json:"at,omitempty"`
}

// VerifiedEntry records a verification event per OKF-SPEC §5.2.
type VerifiedEntry struct {
	By string `yaml:"by" json:"by"`
	At string `yaml:"at,omitempty" json:"at,omitempty"`
}

// VerifiedList handles YAML unmarshaling for both single mapping `{by, at}` and sequence `[{by, at}]`.
type VerifiedList []VerifiedEntry

func (vl *VerifiedList) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.SequenceNode {
		var list []VerifiedEntry
		if err := node.Decode(&list); err != nil {
			return err
		}
		*vl = list
		return nil
	}
	if node.Kind == yaml.MappingNode {
		var single VerifiedEntry
		if err := node.Decode(&single); err != nil {
			return err
		}
		*vl = []VerifiedEntry{single}
		return nil
	}
	return nil
}

// SourceEntry represents an entry in the sources list per OKF-SPEC §5.1.
type SourceEntry struct {
	ID           string `yaml:"id,omitempty" json:"id,omitempty"`
	Resource     string `yaml:"resource" json:"resource"`
	Title        string `yaml:"title,omitempty" json:"title,omitempty"`
	Author       string `yaml:"author,omitempty" json:"author,omitempty"`
	UsageCount   int64  `yaml:"usage_count,omitempty" json:"usage_count,omitempty"`
	LastModified string `yaml:"last_modified,omitempty" json:"last_modified,omitempty"`
}

// UsageWindow represents the window framing usage_count per OKF-SPEC §5.1.
type UsageWindow struct {
	From string `yaml:"from,omitempty" json:"from,omitempty"`
	To   string `yaml:"to,omitempty" json:"to,omitempty"`
}

// ParameterEntry represents a parameter for an Attested Computation concept per OKF-SPEC §10.2.
type ParameterEntry struct {
	Name     string `yaml:"name" json:"name"`
	Type     string `yaml:"type" json:"type"`
	Required bool   `yaml:"required,omitempty" json:"required,omitempty"`
}

// ExecutorInfo represents executor contract per OKF-SPEC §10.2.
type ExecutorInfo struct {
	Resource string   `yaml:"resource,omitempty" json:"resource,omitempty"`
	Receipt  []string `yaml:"receipt,omitempty" json:"receipt,omitempty"`
}

// AttesterInfo represents attester contract per OKF-SPEC §10.2.
type AttesterInfo struct {
	Resource string `yaml:"resource,omitempty" json:"resource,omitempty"`
}

// Frontmatter represents the YAML metadata block at the top of an OKF concept document.
type Frontmatter struct {
	Type            string           `yaml:"type,omitempty" json:"type,omitempty"`                         // The kind of concept (e.g., SQLite Table, Attested Computation)
	Title           string           `yaml:"title,omitempty" json:"title,omitempty"`                      // Display name
	Description     string           `yaml:"description,omitempty" json:"description,omitempty"`          // One-line summary
	Resource        string           `yaml:"resource,omitempty" json:"resource,omitempty"`             // Canonical URI for underlying asset
	Tags            []string         `yaml:"tags,omitempty" json:"tags,omitempty"`                         // Classification tags
	Generated       *GeneratedInfo   `yaml:"generated,omitempty" json:"generated,omitempty"`              // Generation actor and ISO 8601 datetime (§5.2)
	Verified        VerifiedList     `yaml:"verified,omitempty" json:"verified,omitempty"`                // Verification events list (§5.2)
	Sources         []SourceEntry    `yaml:"sources,omitempty" json:"sources,omitempty"`                  // Provenance sources (§5.1)
	UsageWindow     *UsageWindow     `yaml:"usage_window,omitempty" json:"usage_window,omitempty"`        // Shared usage window (§5.1)
	Status          string           `yaml:"status,omitempty" json:"status,omitempty"`                    // draft | stable | deprecated (§5.4)
	StaleAfter      string           `yaml:"stale_after,omitempty" json:"stale_after,omitempty"`            // Absolute staleness date YYYY-MM-DD (§5.5)
	Runtime         string           `yaml:"runtime,omitempty" json:"runtime,omitempty"`                  // Execution runtime for Attested Computation (§10.2)
	Parameters      []ParameterEntry `yaml:"parameters,omitempty" json:"parameters,omitempty"`            // Parameter bindings for Attested Computation (§10.2)
	Computation     string           `yaml:"computation,omitempty" json:"computation,omitempty"`          // External file path for computation (§10.3)
	Executor        *ExecutorInfo    `yaml:"executor,omitempty" json:"executor,omitempty"`                // Executor specification (§10.2)
	Attester        *AttesterInfo    `yaml:"attester,omitempty" json:"attester,omitempty"`                // Attester specification (§10.2)
	Timestamp       string           `yaml:"timestamp,omitempty" json:"timestamp,omitempty"`              // v0.1 legacy ISO 8601 timestamp fallback
	ContentHash     string           `yaml:"content_hash,omitempty" json:"content_hash,omitempty"`        // Structural hash for incremental re-produce
	EnrichedAgainst string           `yaml:"enriched_against,omitempty" json:"enriched_against,omitempty"`// Structural hash description was enriched against
	OKFVersion      string           `yaml:"okf_version,omitempty" json:"okf_version,omitempty"`          // OKF version targeted (permitted in root index.md)
}

// GetTrustTier derives the trust tier per OKF-SPEC §5.3.
func (fm Frontmatter) GetTrustTier() TrustTier {
	if len(fm.Verified) == 0 {
		return TrustTierUnverified
	}
	for _, v := range fm.Verified {
		if strings.HasPrefix(v.By, "human:") {
			return TrustTierHumanReviewed
		}
	}
	return TrustTierMachineConfirmed
}

// IsStale reports whether today >= stale_after per OKF-SPEC §5.5.
func (fm Frontmatter) IsStale(now time.Time) bool {
	if fm.StaleAfter == "" {
		return false
	}
	t, err := time.Parse("2006-01-02", fm.StaleAfter)
	if err != nil {
		return false
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return !today.Before(t)
}

// GetEffectiveTimestamp returns generated.at if set, otherwise legacy timestamp.
func (fm Frontmatter) GetEffectiveTimestamp() string {
	if fm.Generated != nil && fm.Generated.At != "" {
		return fm.Generated.At
	}
	return fm.Timestamp
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

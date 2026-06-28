package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// fileEdit is a computed new content for a single file, not yet written.
type fileEdit struct {
	path    string
	content string
}

// wiringPlan is a set of edits computed in memory. It is written only after every
// edit succeeds, so a missing anchor aborts the whole scaffold before touching disk.
type wiringPlan struct{ edits []fileEdit }

func (p *wiringPlan) write() error {
	for _, e := range p.edits {
		if err := os.WriteFile(e.path, []byte(e.content), 0o644); err != nil {
			return fmt.Errorf("%s: %w", e.path, err)
		}
	}
	return nil
}

// planWiring reads each registration file and computes its edited content. If any
// anchor is missing it returns an error and no plan, so nothing is written.
func planWiring(root, skill, typ, desc, group string) (*wiringPlan, error) {
	type job struct {
		rel  string
		edit func(string) (string, error)
	}
	jobs := []job{
		{"go.work", func(c string) (string, error) { return wireGoWork(c, skill) }},
		{"Makefile", func(c string) (string, error) { return wireMakefile(c, skill) }},
		{"install.sh", func(c string) (string, error) { return wireInstallSh(c, skill) }},
		{"skills.sh.json", func(c string) (string, error) { return wireSkillsJSON(c, skill, group) }},
		{"README.md", func(c string) (string, error) { return wireReadme(c, skill, typ, group) }},
		{"AGENTS.md", func(c string) (string, error) { return wireAgents(c, skill, typ) }},
	}
	var plan wiringPlan
	for _, j := range jobs {
		path := filepath.Join(root, j.rel)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", j.rel, err)
		}
		body, crlf := toLF(string(data))
		out, err := j.edit(body)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", j.rel, err)
		}
		plan.edits = append(plan.edits, fileEdit{path: path, content: fromLF(out, crlf)})
	}
	return &plan, nil
}

// wireGoWork inserts "\t./skills/<skill>" into the use(...) block, alphabetically
// among the existing ./skills/* entries.
func wireGoWork(content, skill string) (string, error) {
	entry := "\t./skills/" + skill
	lines := strings.Split(content, "\n")
	skillsIdx := []int{}
	for i, ln := range lines {
		if strings.HasPrefix(ln, "\t./skills/") {
			if strings.TrimSpace(ln) == "./skills/"+skill {
				return content, nil // already present — idempotent
			}
			skillsIdx = append(skillsIdx, i)
		}
	}
	if len(skillsIdx) == 0 {
		return "", fmt.Errorf("no `./skills/` entries found in use() block")
	}
	insertAt := skillsIdx[len(skillsIdx)-1] + 1 // default: after last skills entry
	for _, i := range skillsIdx {
		if lines[i] > entry { // string compare on the whole "\t./skills/x" line is fine
			insertAt = i
			break
		}
	}
	lines = append(lines[:insertAt], append([]string{entry}, lines[insertAt:]...)...)
	return strings.Join(lines, "\n"), nil
}

var reMakefileSkills = regexp.MustCompile(`(?m)^SKILLS := .*$`)

// wireMakefile appends <skill> to the `SKILLS := ...` line.
func wireMakefile(content, skill string) (string, error) {
	loc := reMakefileSkills.FindString(content)
	if loc == "" {
		return "", fmt.Errorf("`SKILLS :=` line not found")
	}
	if hasWord(loc, skill) {
		return content, nil
	}
	return replaceLine(reMakefileSkills, content, loc+" "+skill), nil
}

var reInstallSkills = regexp.MustCompile(`(?m)^SKILLS="([^"]*)"$`)

// wireInstallSh appends <skill> inside the `SKILLS="..."` assignment.
func wireInstallSh(content, skill string) (string, error) {
	m := reInstallSkills.FindStringSubmatch(content)
	if m == nil {
		return "", fmt.Errorf(`SKILLS="..." line not found`)
	}
	if hasWord(m[1], skill) {
		return content, nil
	}
	return replaceLine(reInstallSkills, content, `SKILLS="`+m[1]+" "+skill+`"`), nil
}

// skillsConfig mirrors skills.sh.json so a struct round-trip preserves field order.
type skillsConfig struct {
	Schema     string       `json:"$schema"`
	NotGrouped string       `json:"notGrouped"`
	Groupings  []skillGroup `json:"groupings"`
}

type skillGroup struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Skills      []string `json:"skills"`
}

// wireSkillsJSON adds <skill> to the named grouping's skills array.
func wireSkillsJSON(content, skill, group string) (string, error) {
	var cfg skillsConfig
	if err := json.Unmarshal([]byte(content), &cfg); err != nil {
		return "", fmt.Errorf("parse JSON: %w", err)
	}
	found := false
	for i := range cfg.Groupings {
		if cfg.Groupings[i].Title != group {
			continue
		}
		found = true
		for _, s := range cfg.Groupings[i].Skills {
			if s == skill {
				return content, nil // idempotent
			}
		}
		cfg.Groupings[i].Skills = append(cfg.Groupings[i].Skills, skill)
	}
	if !found {
		return "", fmt.Errorf("grouping %q not found", group)
	}
	// Use an Encoder with HTML escaping off so "&"/"<"/">" in titles/descriptions
	// (e.g. "Filesystem & Git") are not mangled into &. Encode appends a newline.
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(cfg); err != nil {
		return "", err
	}
	return buf.String(), nil
}

var (
	reConnectorRow = regexp.MustCompile("(?m)^\\| `okf-[a-z0-9-]+` \\|.*$")
)

// wireReadme adds a row to the "Available Connectors" table and appends the skill to
// the matching §8 group-table row.
func wireReadme(content, skill, typ, group string) (string, error) {
	if strings.Contains(content, "| `"+skill+"` |") {
		// connector row already present; assume fully wired (idempotent)
		return content, nil
	}
	// 1. Insert a connector-table row after the last existing connector row.
	rowLocs := reConnectorRow.FindAllStringIndex(content, -1)
	if rowLocs == nil {
		return "", fmt.Errorf("Available Connectors table rows not found")
	}
	last := rowLocs[len(rowLocs)-1]
	newRow := "\n| `" + skill + "` | " + typ + " | — |"
	content = content[:last[1]] + newRow + content[last[1]:]

	// 2. Append to the §8 group-table row.
	reGroupRow := regexp.MustCompile("(?m)^\\| " + regexp.QuoteMeta(group) + " \\| (.*) \\|$")
	gm := reGroupRow.FindStringSubmatch(content)
	if gm == nil {
		return "", fmt.Errorf("group-table row %q not found", group)
	}
	content = reGroupRow.ReplaceAllStringFunc(content, func(m string) string {
		return "| " + group + " | " + gm[1] + ", `" + skill + "` |"
	})
	return content, nil
}

var reAgentsFirstSkill = regexp.MustCompile(`(?m)^│   ├── okf-[a-z0-9-]+/.*$`)

// wireAgents inserts a tree line for the new skill under the skills/ block in §1.
func wireAgents(content, skill, typ string) (string, error) {
	if strings.Contains(content, "├── "+skill+"/") || strings.Contains(content, "└── "+skill+"/") {
		return content, nil
	}
	loc := reAgentsFirstSkill.FindStringIndex(content)
	if loc == nil {
		return "", fmt.Errorf("skills/ tree entries not found in §1")
	}
	line := "\n│   ├── " + skill + "/                # " + typ + " connector"
	return content[:loc[1]] + line + content[loc[1]:], nil
}

// hasWord reports whether s contains skill as a whitespace-delimited word.
func hasWord(s, skill string) bool {
	for _, f := range strings.Fields(s) {
		if f == skill {
			return true
		}
	}
	return false
}

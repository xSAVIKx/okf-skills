package okf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// logTitle is the bundle-root change log's top-level heading.
const logTitle = "# Change Log"

// AppendLogEntry records a change to the bundle-root log.md in OKF-SPEC §7 format:
// newest-first "## YYYY-MM-DD" date headings, each with "* **<Kind>**: <message>"
// bullet lines. Entries for the same date group under one heading (newest within
// the day first); a new date inserts a fresh heading at the top, above older days.
//
// date is supplied by the caller (connectors pass time.Now().Format("2006-01-02"))
// so this function stays deterministic and testable.
func AppendLogEntry(bundleDir, date, kind, message string) error {
	path := filepath.Join(bundleDir, "log.md")
	line := fmt.Sprintf("* **%s**: %s", kind, message)
	heading := "## " + date

	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if strings.TrimSpace(string(data)) == "" {
		out := logTitle + "\n\n" + heading + "\n\n" + line + "\n"
		return os.WriteFile(path, []byte(out), 0644)
	}

	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")

	// Locate the first (newest) date heading.
	firstDate := -1
	for i, ln := range lines {
		if strings.HasPrefix(strings.TrimSpace(ln), "## ") {
			firstDate = i
			break
		}
	}

	var out []string
	if firstDate >= 0 && strings.TrimSpace(lines[firstDate]) == heading {
		// Same day: insert the new bullet directly under the heading (newest first).
		insertAt := firstDate + 1
		if insertAt < len(lines) && strings.TrimSpace(lines[insertAt]) == "" {
			insertAt++
		}
		out = append(out, lines[:insertAt]...)
		out = append(out, line)
		out = append(out, lines[insertAt:]...)
	} else {
		// New day (or no date headings yet): insert a fresh block at the top of the
		// date list, after any title/preamble.
		insertAt := firstDate
		if insertAt < 0 {
			insertAt = len(lines)
		}
		block := []string{heading, "", line, ""}
		out = append(out, lines[:insertAt]...)
		out = append(out, block...)
		out = append(out, lines[insertAt:]...)
	}

	joined := strings.TrimRight(strings.Join(out, "\n"), "\n") + "\n"
	return os.WriteFile(path, []byte(joined), 0644)
}

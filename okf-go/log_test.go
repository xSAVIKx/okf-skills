package okf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendLogEntry_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	if err := AppendLogEntry(dir, "2026-06-14", "Creation", "Established [orders](/tables/orders.md)."); err != nil {
		t.Fatalf("append failed: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "log.md"))
	got := string(data)
	if !strings.Contains(got, "## 2026-06-14") || !strings.Contains(got, "* **Creation**: Established [orders](/tables/orders.md).") {
		t.Fatalf("unexpected log content:\n%s", got)
	}
}

func TestAppendLogEntry_SameDayGroupsNewestFirst(t *testing.T) {
	dir := t.TempDir()
	_ = AppendLogEntry(dir, "2026-06-14", "Creation", "first")
	_ = AppendLogEntry(dir, "2026-06-14", "Update", "second")

	data, _ := os.ReadFile(filepath.Join(dir, "log.md"))
	got := string(data)
	if strings.Count(got, "## 2026-06-14") != 1 {
		t.Fatalf("same-day entries must share one heading:\n%s", got)
	}
	if strings.Index(got, "second") > strings.Index(got, "first") {
		t.Fatalf("newest entry must appear first within a day:\n%s", got)
	}
}

func TestAppendLogEntry_NewDayInsertsHeadingAtTop(t *testing.T) {
	dir := t.TempDir()
	_ = AppendLogEntry(dir, "2026-06-13", "Creation", "old day")
	_ = AppendLogEntry(dir, "2026-06-14", "Update", "new day")

	data, _ := os.ReadFile(filepath.Join(dir, "log.md"))
	got := string(data)
	if strings.Index(got, "## 2026-06-14") > strings.Index(got, "## 2026-06-13") {
		t.Fatalf("newest day heading must be above older ones:\n%s", got)
	}
}

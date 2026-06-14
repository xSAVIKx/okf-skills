package okf

import (
	"regexp"
	"strings"
)

// placeholderPatterns is the catalog of connector-generated placeholder
// descriptions, anchored so a real description that merely mentions a type word is
// not misclassified. Each entry corresponds to a connector's deterministic
// fallback description; keeping them here (next to the OKF types) means a new
// connector adds its pattern in one place and every consumer (coverage report,
// future okf-lint) stays precise rather than heuristic.
var placeholderPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^SQLite (table|view) .+$`),
	regexp.MustCompile(`^MySQL (table|view) .+$`),
	regexp.MustCompile(`^PostgreSQL (table|view) .+$`),
	regexp.MustCompile(`^BigQuery (table|view) .+$`),
	regexp.MustCompile(`^File .+$`),
	regexp.MustCompile(`^Directory .+$`),
	regexp.MustCompile(`^Git file .+$`),
	regexp.MustCompile(`^Git directory .+$`),
	regexp.MustCompile(`^No description available$`),
}

// IsPlaceholderDescription reports whether a concept's description is an
// unenriched connector placeholder (or empty/whitespace). It is the precise,
// deterministic "not yet enriched" signal the coverage report and triage rely on.
func IsPlaceholderDescription(desc string) bool {
	t := strings.TrimSpace(desc)
	if t == "" {
		return true
	}
	for _, re := range placeholderPatterns {
		if re.MatchString(t) {
			return true
		}
	}
	return false
}

package okf

import (
	"regexp"
	"sort"
	"strings"
)

// LowCardinalityN is the threshold at or below which a column's distinct values
// are considered an enumerable domain worth emitting literally.
const LowCardinalityN = 12

// semantic detection regexes — deliberately conservative; a false tag is worse
// than no tag.
var (
	reEmail    = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
	reUUID     = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	reISOTime  = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}([ T]\d{2}:\d{2}(:\d{2})?(\.\d+)?(Z|[+-]\d{2}:?\d{2})?)?$`)
	reMonetary = regexp.MustCompile(`^[\$€£¥]\s?-?\d[\d,]*(\.\d{2})?$|^-?\d[\d,]*\.\d{2}$`)
	reFKName   = regexp.MustCompile(`(?i)(^|_)id$|_id$|^id[A-Z]|Id$`)
	reIntValue = regexp.MustCompile(`^-?\d+$`)
	booleanSet = map[string]bool{"true": true, "false": true, "0": true, "1": true, "t": true, "f": true, "yes": true, "no": true, "y": true, "n": true}
)

// DetectSemanticType inspects non-null sample values and returns a single
// lowercase semantic tag (email, uuid, iso-timestamp, monetary, boolean) when a
// strong majority (>=90%) of the non-empty values match, or "" when nothing
// matches with confidence. Pure: no I/O, no LLM. Order is most-specific-first.
//
// Enum and fk-ish are intentionally NOT decided here — they depend on the column's
// distinct count and name, which the connector layers on via ClassifyColumn.
func DetectSemanticType(samples []string) string {
	vals := make([]string, 0, len(samples))
	for _, s := range samples {
		s = strings.TrimSpace(s)
		if s != "" {
			vals = append(vals, s)
		}
	}
	if len(vals) == 0 {
		return ""
	}

	majority := func(pred func(string) bool) bool {
		n := 0
		for _, v := range vals {
			if pred(v) {
				n++
			}
		}
		return float64(n) >= 0.9*float64(len(vals))
	}

	switch {
	case majority(func(v string) bool { return reEmail.MatchString(v) }):
		return "email"
	case majority(func(v string) bool { return reUUID.MatchString(v) }):
		return "uuid"
	case majority(func(v string) bool { return reISOTime.MatchString(v) }):
		return "iso-timestamp"
	case majority(func(v string) bool { return reMonetary.MatchString(v) }):
		return "monetary"
	case majority(func(v string) bool { return booleanSet[strings.ToLower(v)] }):
		return "boolean"
	}
	return ""
}

// ClassifyColumn combines the value-based verdict of DetectSemanticType with the
// connector's structural knowledge — the column name and distinct count — to
// produce the final Semantic tag and, for low-cardinality columns, the sorted
// literal value set to render. distinctVals are up to LowCardinalityN+1 distinct
// values the connector already read; distinct is the true distinct count.
//
//   - A value-based tag (email/uuid/…) wins.
//   - Otherwise a column with 0 < distinct <= LowCardinalityN is an "enum".
//   - Otherwise an id-shaped name with all-integer values is advisory "fk-ish"
//     (the authoritative FK signal is the deterministic edge, not this hint).
//
// Values is the sorted distinct set when distinct <= LowCardinalityN (and the
// connector actually supplied them), else nil.
func ClassifyColumn(colName string, distinctVals []string, distinct int64) (semantic string, values []string) {
	semantic = DetectSemanticType(distinctVals)
	if semantic == "" {
		switch {
		case distinct > 0 && distinct <= LowCardinalityN:
			semantic = "enum"
		case reFKName.MatchString(colName) && allInteger(distinctVals):
			semantic = "fk-ish"
		}
	}

	if distinct > 0 && distinct <= LowCardinalityN && len(distinctVals) > 0 && int64(len(distinctVals)) <= LowCardinalityN {
		values = append([]string{}, distinctVals...)
		sort.Strings(values)
	}
	return semantic, values
}

// allInteger reports whether every non-empty value is an integer literal.
func allInteger(vals []string) bool {
	any := false
	for _, v := range vals {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		any = true
		if !reIntValue.MatchString(v) {
			return false
		}
	}
	return any
}

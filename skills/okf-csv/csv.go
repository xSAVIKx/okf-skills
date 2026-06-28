package main

import (
	"encoding/csv"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/xSAVIKx/okf-skills/okf-go"
)

// inferenceCap bounds how many rows are scanned for type inference, so a huge CSV
// stays cheap. Profiling (over all rows) is gated separately behind --profile.
const inferenceCap = 1000

// readCSV reads the header row and data rows of a CSV file. Ragged rows are
// tolerated (short rows are treated as having empty trailing cells).
func readCSV(path string) (header []string, rows [][]string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		return nil, nil, err
	}
	if len(records) == 0 {
		return nil, nil, nil
	}
	return records[0], records[1:], nil
}

var boolValues = map[string]bool{
	"true": true, "false": true, "yes": true, "no": true, "t": true, "f": true,
}

var dateLayouts = []string{"2006-01-02", time.RFC3339, "2006-01-02 15:04:05"}

func isDate(v string) bool {
	for _, l := range dateLayouts {
		if _, err := time.Parse(l, v); err == nil {
			return true
		}
	}
	return false
}

// inferType classifies a column from its values: integer, number, boolean, date, or
// string. Empty values are ignored; a column with no non-empty values is "string".
func inferType(samples []string) string {
	var vals []string
	for _, s := range samples {
		if s = strings.TrimSpace(s); s != "" {
			vals = append(vals, s)
		}
	}
	if len(vals) == 0 {
		return "string"
	}
	allInt, allFloat, allBool, allDate := true, true, true, true
	for _, v := range vals {
		if allInt {
			if _, err := strconv.ParseInt(v, 10, 64); err != nil {
				allInt = false
			}
		}
		if allFloat {
			if _, err := strconv.ParseFloat(v, 64); err != nil {
				allFloat = false
			}
		}
		if allBool && !boolValues[strings.ToLower(v)] {
			allBool = false
		}
		if allDate && !isDate(v) {
			allDate = false
		}
	}
	switch {
	case allInt:
		return "integer"
	case allFloat:
		return "number"
	case allBool:
		return "boolean"
	case allDate:
		return "date"
	default:
		return "string"
	}
}

// columnTypes infers each column's type from up to inferenceCap rows.
func columnTypes(header []string, rows [][]string) []string {
	types := make([]string, len(header))
	for i := range header {
		var col []string
		for r, row := range rows {
			if r >= inferenceCap {
				break
			}
			if i < len(row) {
				col = append(col, row[i])
			}
		}
		types[i] = inferType(col)
	}
	return types
}

// columnProfiles computes a deterministic ColumnProfile per column over all rows.
// Numeric columns get numeric min/max; others get lexical min/max.
func columnProfiles(header []string, rows [][]string, types []string) []okf.ColumnProfile {
	profs := make([]okf.ColumnProfile, len(header))
	for i, name := range header {
		numeric := i < len(types) && (types[i] == "integer" || types[i] == "number")
		distinct := map[string]bool{}
		var nonNull, null int64
		var nonEmpty []string
		var minS, maxS string
		var minF, maxF float64
		haveF := false
		for _, row := range rows {
			v := ""
			if i < len(row) {
				v = strings.TrimSpace(row[i])
			}
			if v == "" {
				null++
				continue
			}
			nonNull++
			nonEmpty = append(nonEmpty, v)
			distinct[v] = true
			if numeric {
				if f, err := strconv.ParseFloat(v, 64); err == nil {
					if !haveF || f < minF {
						minF = f
					}
					if !haveF || f > maxF {
						maxF = f
					}
					haveF = true
				}
			} else {
				if minS == "" || v < minS {
					minS = v
				}
				if maxS == "" || v > maxS {
					maxS = v
				}
			}
		}
		var values []string
		if n := len(distinct); n > 0 && n <= okf.LowCardinalityN {
			for v := range distinct {
				values = append(values, v)
			}
			sort.Strings(values)
		}
		p := okf.ColumnProfile{
			Column: name, NonNull: nonNull, Null: null,
			Distinct: int64(len(distinct)),
			Semantic: okf.DetectSemanticType(nonEmpty),
			Values:   values,
		}
		if numeric && haveF {
			p.Min = strconv.FormatFloat(minF, 'g', -1, 64)
			p.Max = strconv.FormatFloat(maxF, 'g', -1, 64)
		} else {
			p.Min, p.Max = minS, maxS
		}
		profs[i] = p
	}
	return profs
}

// renderColumns renders the "# Columns" section for a CSV's header + inferred types.
func renderColumns(header, types []string) string {
	var b strings.Builder
	b.WriteString("# Columns\n\n")
	b.WriteString("| Name | Type |\n")
	b.WriteString("| --- | --- |\n")
	for i, name := range header {
		typ := "string"
		if i < len(types) {
			typ = types[i]
		}
		b.WriteString("| " + okf.SanitizeCell(name) + " | " + okf.SanitizeCell(typ) + " |\n")
	}
	return b.String()
}

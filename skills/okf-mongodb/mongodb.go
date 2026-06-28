package main

import (
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/xSAVIKx/okf-skills/okf-go"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// fieldProfile is an inferred top-level field of a collection.
type fieldProfile struct {
	Name        string
	Type        string // BSON/JSON type, or a "a|b" union when mixed across docs
	PresencePct int    // % of sampled documents that contain the field
}

// bsonType returns a stable, human type name for a sampled value.
func bsonType(v interface{}) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case int, int32, int64, float32, float64:
		return "number"
	case string:
		return "string"
	case primitive.ObjectID:
		return "objectId"
	case primitive.DateTime, time.Time:
		return "date"
	case primitive.A, []interface{}:
		return "array"
	case primitive.M, primitive.D, map[string]interface{}:
		return "object"
	default:
		return "string"
	}
}

// inferFields samples documents and returns one profile per top-level field,
// deterministically ordered by field name. Mixed types are rendered as a sorted
// union ("number|string"). Presence is the rounded percentage of docs with the field.
func inferFields(docs []map[string]interface{}) []fieldProfile {
	type stat struct {
		present int
		types   map[string]bool
	}
	stats := map[string]*stat{}
	for _, d := range docs {
		for k, v := range d {
			s := stats[k]
			if s == nil {
				s = &stat{types: map[string]bool{}}
				stats[k] = s
			}
			s.present++
			s.types[bsonType(v)] = true
		}
	}
	names := make([]string, 0, len(stats))
	for n := range stats {
		names = append(names, n)
	}
	sort.Strings(names)

	total := len(docs)
	var out []fieldProfile
	for _, n := range names {
		s := stats[n]
		types := make([]string, 0, len(s.types))
		for t := range s.types {
			types = append(types, t)
		}
		sort.Strings(types)
		presence := 0
		if total > 0 {
			presence = int(math.Round(100 * float64(s.present) / float64(total)))
		}
		out = append(out, fieldProfile{Name: n, Type: strings.Join(types, "|"), PresencePct: presence})
	}
	return out
}

// renderColumns renders the "# Columns" section for inferred fields.
func renderColumns(fields []fieldProfile) string {
	var b strings.Builder
	b.WriteString("# Columns\n\n")
	b.WriteString("| Name | Type | Presence |\n")
	b.WriteString("| --- | --- | --- |\n")
	for _, f := range fields {
		fmt.Fprintf(&b, "| %s | %s | %d%% |\n", okf.SanitizeCell(f.Name), okf.SanitizeCell(f.Type), f.PresencePct)
	}
	return b.String()
}

// stripCreds removes any userinfo (username:password) from a MongoDB URI so it is
// safe to record in a concept's Resource field.
func stripCreds(uri string) string {
	u, err := url.Parse(uri)
	if err != nil || u.User == nil {
		return uri
	}
	u.User = nil
	return u.String()
}

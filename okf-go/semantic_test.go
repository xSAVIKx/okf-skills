package okf

import (
	"strings"
	"testing"
)

func TestDetectSemanticType(t *testing.T) {
	cases := []struct {
		name    string
		samples []string
		want    string
	}{
		{"email", []string{"a@x.com", "b@y.org", "c@z.net"}, "email"},
		{"uuid", []string{"123e4567-e89b-12d3-a456-426614174000", "00000000-0000-0000-0000-000000000001"}, "uuid"},
		{"iso date", []string{"2019-01-01", "2026-06-14"}, "iso-timestamp"},
		{"iso datetime", []string{"2019-01-01T10:00:00Z", "2026-06-14 23:59:59"}, "iso-timestamp"},
		{"monetary symbol", []string{"$10.00", "$1,234.56"}, "monetary"},
		{"monetary plain", []string{"10.00", "1234.56"}, "monetary"},
		{"boolean", []string{"true", "false", "true"}, "boolean"},
		{"boolean 01", []string{"0", "1", "1", "0"}, "boolean"},
		{"plain text", []string{"alice", "bob", "carol"}, ""},
		{"empty", nil, ""},
		{"all blank", []string{"", "   "}, ""},
		{"one stray does not flip", []string{"a@x.com", "b@y.org", "c@z.net", "d@w.io", "e@v.co", "f@u.dev", "g@t.app", "h@s.net", "i@r.org", "notanemail"}, "email"},
		{"too many strays no tag", []string{"a@x.com", "nope", "also-nope"}, ""},
	}
	for _, c := range cases {
		if got := DetectSemanticType(c.samples); got != c.want {
			t.Errorf("%s: DetectSemanticType=%q want %q", c.name, got, c.want)
		}
	}
}

func TestClassifyColumn_EnumAndValues(t *testing.T) {
	sem, vals := ClassifyColumn("status", []string{"shipped", "pending", "cancelled"}, 3)
	if sem != "enum" {
		t.Fatalf("expected enum, got %q", sem)
	}
	if strings.Join(vals, ",") != "cancelled,pending,shipped" {
		t.Fatalf("expected sorted values, got %v", vals)
	}
}

func TestClassifyColumn_SemanticWinsOverEnum(t *testing.T) {
	sem, vals := ClassifyColumn("contact", []string{"a@x.com", "b@y.com"}, 2)
	if sem != "email" {
		t.Fatalf("expected email to win over enum, got %q", sem)
	}
	// low-cardinality so values still emitted
	if len(vals) != 2 {
		t.Fatalf("expected 2 values, got %v", vals)
	}
}

func TestClassifyColumn_FKish(t *testing.T) {
	sem, vals := ClassifyColumn("customer_id", []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13"}, 5000)
	if sem != "fk-ish" {
		t.Fatalf("expected fk-ish, got %q", sem)
	}
	if vals != nil {
		t.Fatalf("high-cardinality column must not emit values, got %v", vals)
	}
}

func TestClassifyColumn_HighCardinalityNoValues(t *testing.T) {
	_, vals := ClassifyColumn("name", []string{"a", "b", "c"}, 9999)
	if vals != nil {
		t.Fatalf("expected nil values for high-cardinality, got %v", vals)
	}
}

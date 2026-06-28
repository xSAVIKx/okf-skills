package main

import (
	"strings"
	"testing"
)

func TestInferFields(t *testing.T) {
	docs := []map[string]interface{}{
		{"_id": "x1", "total": 10.0, "active": true},
		{"_id": "x2", "total": 20.0},                              // missing active
		{"_id": "x3", "total": "n/a", "tags": []interface{}{"a"}}, // total mixed type
	}
	fields := inferFields(docs)
	by := map[string]fieldProfile{}
	for _, f := range fields {
		by[f.Name] = f
	}

	// _id present in all 3 -> 100%
	if by["_id"].PresencePct != 100 {
		t.Errorf("_id presence = %d, want 100", by["_id"].PresencePct)
	}
	// active in 1 of 3 -> 33%
	if by["active"].PresencePct != 33 {
		t.Errorf("active presence = %d, want 33", by["active"].PresencePct)
	}
	// total is number in 2 docs, string in 1 -> union "number|string"
	if by["total"].Type != "number|string" {
		t.Errorf("total type = %q, want number|string", by["total"].Type)
	}
	if by["tags"].Type != "array" {
		t.Errorf("tags type = %q, want array", by["tags"].Type)
	}
	// deterministic field order (sorted by name)
	var names []string
	for _, f := range fields {
		names = append(names, f.Name)
	}
	if strings.Join(names, ",") != "_id,active,tags,total" {
		t.Errorf("field order = %v, want sorted", names)
	}
}

func TestBsonType(t *testing.T) {
	cases := []struct {
		v    interface{}
		want string
	}{
		{nil, "null"},
		{true, "boolean"},
		{int64(5), "number"},
		{3.14, "number"},
		{"hi", "string"},
		{[]interface{}{1, 2}, "array"},
		{map[string]interface{}{"a": 1}, "object"},
	}
	for _, c := range cases {
		if got := bsonType(c.v); got != c.want {
			t.Errorf("bsonType(%v) = %q, want %q", c.v, got, c.want)
		}
	}
}

func TestStripCreds(t *testing.T) {
	cases := map[string]string{
		"mongodb://user:pass@host:27017/db": "mongodb://host:27017/db",
		"mongodb://host:27017":              "mongodb://host:27017",
		"mongodb+srv://u:p@cluster.example": "mongodb+srv://cluster.example",
	}
	for in, want := range cases {
		if got := stripCreds(in); got != want {
			t.Errorf("stripCreds(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRenderColumns(t *testing.T) {
	out := renderColumns([]fieldProfile{
		{Name: "_id", Type: "objectId", PresencePct: 100},
		{Name: "total", Type: "number", PresencePct: 98},
	})
	for _, want := range []string{"# Columns", "| Name | Type | Presence |", "| _id | objectId | 100% |", "| total | number | 98% |"} {
		if !strings.Contains(out, want) {
			t.Errorf("renderColumns missing %q:\n%s", want, out)
		}
	}
}

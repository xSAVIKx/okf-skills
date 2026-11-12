package main

import "testing"

func TestCanonicalURL(t *testing.T) {
	cases := []struct{ base, file, want string }{
		{"", "index.html", ""},
		{"https://example.org/shop", "index.html", "https://example.org/shop/"},
		{"https://example.org/shop/", "index.html", "https://example.org/shop/"},
		{"https://example.org/shop", "full.html", "https://example.org/shop/full.html"},
	}
	for _, c := range cases {
		if got := canonicalURL(c.base, c.file); got != c.want {
			t.Errorf("canonicalURL(%q, %q) = %q, want %q", c.base, c.file, got, c.want)
		}
	}
}

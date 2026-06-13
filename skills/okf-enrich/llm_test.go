package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIGenerator_Describe(t *testing.T) {
	var gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"  A table of users.  "}}]}`)
	}))
	defer srv.Close()
	g := NewOpenAIGenerator(srv.URL, "test-model", "secret")
	out, err := g.Describe(context.Background(), "describe users")
	if err != nil {
		t.Fatal(err)
	}
	if out != "A table of users." {
		t.Fatalf("out=%q", out)
	}
	if gotAuth != "Bearer secret" {
		t.Fatalf("auth=%q", gotAuth)
	}
	if !strings.Contains(gotBody, "test-model") || !strings.Contains(gotBody, "describe users") {
		t.Fatalf("body=%q", gotBody)
	}
}

func TestOpenAIGenerator_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		io.WriteString(w, `{"error":{"message":"bad key"}}`)
	}))
	defer srv.Close()
	g := NewOpenAIGenerator(srv.URL, "m", "x")
	if _, err := g.Describe(context.Background(), "p"); err == nil {
		t.Fatal("expected error on 401")
	}
}

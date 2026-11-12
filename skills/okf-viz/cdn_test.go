package main

import (
	"crypto/sha512"
	"encoding/base64"
	"io"
	"net/http"
	"os"
	"regexp"
	"testing"
	"time"
)

// cdnScriptRE matches a CDN URL and its SRI hash in a tag from cdnTags. The
// optional "data-" prefix also accepts a tag that the page loads later.
var cdnScriptRE = regexp.MustCompile(`(?:data-)?src="([^"]+)"[^>]*?(?:data-)?integrity="([^"]+)"`)

// exactVersionRE matches a jsDelivr npm URL with an exact x.y.z version.
var exactVersionRE = regexp.MustCompile(`^https://cdn\.jsdelivr\.net/npm/(?:@[a-z0-9._-]+/)?[a-z0-9._-]+@\d+\.\d+\.\d+/[^@]+$`)

type cdnEntry struct{ url, integrity string }

func parseCDNTags(t *testing.T) []cdnEntry {
	t.Helper()
	var out []cdnEntry
	for _, m := range cdnScriptRE.FindAllStringSubmatch(cdnTags(), -1) {
		out = append(out, cdnEntry{m[1], m[2]})
	}
	if len(out) != len(vendorOrder) {
		t.Fatalf("cdnTags has %d tags with SRI, want %d (one for each vendored library)", len(out), len(vendorOrder))
	}
	return out
}

// TestCDNTagsPinned checks that each CDN URL has an exact version and a valid
// sha384 SRI hash. It does not use the network.
func TestCDNTagsPinned(t *testing.T) {
	for _, e := range parseCDNTags(t) {
		if !exactVersionRE.MatchString(e.url) {
			t.Errorf("%s: URL must pin an exact x.y.z version", e.url)
		}
		const prefix = "sha384-"
		if len(e.integrity) <= len(prefix) || e.integrity[:len(prefix)] != prefix {
			t.Errorf("%s: integrity %q must start with %q", e.url, e.integrity, prefix)
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(e.integrity[len(prefix):])
		if err != nil || len(raw) != sha512.Size384 {
			t.Errorf("%s: integrity %q is not a base64 sha384 digest", e.url, e.integrity)
		}
	}
}

// TestCDNTagsLive downloads each CDN URL and compares its sha384 digest with
// the pinned SRI hash. It uses the network, so it runs only when
// OKF_VIZ_CDN_CHECK=1 is set.
func TestCDNTagsLive(t *testing.T) {
	if os.Getenv("OKF_VIZ_CDN_CHECK") != "1" {
		t.Skip("set OKF_VIZ_CDN_CHECK=1 to check the SRI hashes against the CDN")
	}
	client := &http.Client{Timeout: 60 * time.Second}
	for _, e := range parseCDNTags(t) {
		resp, err := client.Get(e.url)
		if err != nil {
			t.Errorf("%s: %v", e.url, err)
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil || resp.StatusCode != http.StatusOK {
			t.Errorf("%s: status %d, err %v", e.url, resp.StatusCode, err)
			continue
		}
		sum := sha512.Sum384(body)
		if got := "sha384-" + base64.StdEncoding.EncodeToString(sum[:]); got != e.integrity {
			t.Errorf("%s: CDN serves %s, cdn.go pins %s", e.url, got, e.integrity)
		}
	}
}

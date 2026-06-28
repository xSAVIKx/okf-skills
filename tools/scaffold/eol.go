package main

import "strings"

// toLF normalizes CRLF content to LF for matching/editing and reports whether the
// original used CRLF, so the file's line-ending style can be restored on write.
// This keeps every editor's regex LF-only while preserving the repo's existing EOL
// convention (a Windows checkout is typically CRLF).
func toLF(content string) (body string, crlf bool) {
	if strings.Contains(content, "\r\n") {
		return strings.ReplaceAll(content, "\r\n", "\n"), true
	}
	return content, false
}

// fromLF restores CRLF line endings when the original file used them.
func fromLF(body string, crlf bool) string {
	if crlf {
		return strings.ReplaceAll(body, "\n", "\r\n")
	}
	return body
}

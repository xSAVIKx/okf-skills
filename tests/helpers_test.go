package tests

import (
	"fmt"
	"net"
	"path/filepath"
	"runtime"
	"time"
)

// getBinaryPath returns the path to the compiled skill binary.
func getBinaryPath(skillName string) string {
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	return filepath.Join("..", "skills", skillName, skillName+ext)
}

// isPortOpen returns true if a TCP port is open.
func isPortOpen(host string, port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// bytesBuffer is a helper to implement a thread-safe bytes.Buffer for stdout/stderr capturing.
type bytesBuffer struct {
	buf []byte
}

func (b *bytesBuffer) Write(p []byte) (n int, err error) {
	b.buf = append(b.buf, p...)
	return len(p), nil
}

func (b *bytesBuffer) String() string {
	return string(b.buf)
}

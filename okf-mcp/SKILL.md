---
name: okf-mcp
description: Generic Model Context Protocol (MCP) server that discovers installed okf-* skill binaries and exposes their produce/ingest commands as MCP tools for any MCP-capable harness (Claude Code, Gemini CLI, etc.). Use when wiring OKF Skills into an agent over MCP instead of invoking each connector binary directly.
license: Apache-2.0
compatibility: Requires the Go toolchain (1.24+) to build the server binary, plus installed okf-* skill binaries to discover and expose.
metadata:
  version: "0.5.0"
  author: Yurii Serhiichuk
  tags: "okf, mcp, registry, tools"
---

# OKF MCP Server

`okf-mcp` is a generic [Model Context Protocol](https://modelcontextprotocol.io) server. On startup it discovers installed `okf-*` skill binaries, runs `<skill> schema` on each, and exposes every skill command (except `schema` itself) as an MCP tool named `<skill>__<command>` (e.g. `okf-sqlite__produce`). Any MCP-capable harness can then drive all of OKF Skills without per-skill glue.

## How it works

1. **Discovery** — scans `--skills-dir`, else `$OKF_SKILLS_DIR`, else every `$PATH` entry, for `okf-*` executables.
2. **Reflection** — reads each skill's machine-readable `schema` output and turns its flags into a JSON-Schema tool input.
3. **Invocation** — a tool call is translated back into the skill's CLI (`<command> --flag value ...`) and run with a per-call timeout. Flags that advertise an `env` binding (e.g. database passwords) are passed via the child environment, never argv. The skill's stdout is returned as the tool result.

## Install

Install the published server binary (Go 1.24+) — no clone needed:

```bash
go install github.com/xSAVIKx/okf-skills/okf-mcp@v0.1.0
```

This drops an `okf-mcp` binary into `$(go env GOPATH)/bin`. Alternatively, build from a clone: `cd okf-mcp && go build -o okf-mcp .`

## Running

```bash
okf-mcp                      # discover skills on PATH, serve over stdio
okf-mcp --skills-dir ./bin   # discover skills in a specific directory
okf-mcp --timeout 2m         # per-invocation timeout (default 5m)
```

Diagnostics are written to stderr; stdout is the MCP protocol channel.

## Configuring a harness

Point any MCP client at the `okf-mcp` command over stdio. Example (generic MCP client config):

```json
{
  "mcpServers": {
    "okf": { "command": "okf-mcp", "args": ["--skills-dir", "/path/to/okf/bin"] }
  }
}
```

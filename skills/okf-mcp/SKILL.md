---
name: okf-mcp
description: Generic MCP server that discovers installed okf-* skills and exposes their produce/ingest commands as MCP tools for any MCP-capable harness.
version: 0.1.0
author: Yurii Serhiichuk
tags:
  - okf
  - mcp
  - registry
  - tools
---

# OKF MCP Server

`okf-mcp` is a generic [Model Context Protocol](https://modelcontextprotocol.io) server. On startup it discovers installed `okf-*` skill binaries, runs `<skill> schema` on each, and exposes every skill command (except `schema` itself) as an MCP tool named `<skill>__<command>` (e.g. `okf-sqlite__produce`). Any MCP-capable harness can then drive the whole OKF skills registry without per-skill glue.

## How it works

1. **Discovery** — scans `--skills-dir`, else `$OKF_SKILLS_DIR`, else every `$PATH` entry, for `okf-*` executables.
2. **Reflection** — reads each skill's machine-readable `schema` output and turns its flags into a JSON-Schema tool input.
3. **Invocation** — a tool call is translated back into the skill's CLI (`<command> --flag value ...`) and run with a per-call timeout. Flags that advertise an `env` binding (e.g. database passwords) are passed via the child environment, never argv. The skill's stdout is returned as the tool result.

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

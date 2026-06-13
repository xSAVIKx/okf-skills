---
name: okf-reader
description: Guidance for AI agents on how to parse, traverse, and query Open Knowledge Format (OKF) bundles efficiently — minimizing token usage and avoiding slow recursive directory walks. Use when reading, navigating, analyzing, or answering questions about an existing OKF bundle. Instructions-only; no binary required.
license: Apache-2.0
metadata:
  version: "0.1.0"
  author: Yurii Serhiichuk
  tags: "okf, knowledge-catalog, agent-guidance, documentation, prompt-engineering"
---

# OKF Bundle Reader Guidance Skill

This skill provides a set of procedural rules and best practices for AI agents (such as Claude Code, Cursor, Copilot, or others) to read and traverse Open Knowledge Format (OKF) bundles efficiently. Following these rules reduces token consumption, speeds up execution, and prevents recursive directory-walking overhead.

## When to Use

This skill should be loaded by the agent whenever it is tasked with reading, parsing, querying, or analyzing an existing OKF bundle.

## Instructions for Agents

When interacting with an OKF bundle, you MUST adhere to the following optimization protocols:

### 1. Progressive Bundle Discovery (Index First)
- **Rule**: NEVER run a recursive read or load all markdown files in the bundle at startup.
- **Protocol**:
  1. Check for the existence of `index.md` at the bundle root.
  2. If `index.md` exists, read it first. It acts as the directory listing and schema description, containing links to all table concepts.
  3. Use the links in `index.md` to map out the names of tables/concepts rather than performing a directory listing.

### 2. Direct Concept Routing
- **Rule**: Route file reads directly by utilizing the standard OKF folder structure.
- **Protocol**:
  - SQLite, MySQL, and PostgreSQL concepts are organized under the `tables/` directory.
  - If a user asks about a table named `customers`, open `tables/customers.md` directly. Do not list the directory or open other files.

### 3. Frontmatter-Only Parsing
- **Rule**: If you only need to identify metadata, types, or resource locations (e.g., routing tables), do not parse or read the markdown bodies.
- **Protocol**:
  - Read only the top YAML frontmatter block (everything between the first and second `---`).
  - Parse the `type`, `title`, and `resource` fields to filter or match tables.

### 4. Fast Schema Searching (Using Grep)
- **Rule**: If searching for a specific column or keyword across the entire bundle, do not open and read each markdown file sequentially.
- **Protocol**:
  - Propose a search command (like `grep` or `ripgrep`) to find the column name across all files in the bundle directory.
  - Only open the files containing matches returned by the search tool.

### 5. Concept Relationship Traversal
- **Rule**: Do not guess schema relationships. Look for standard markdown links in the table concepts.
- **Protocol**:
  - Scan the body of `tables/<table_name>.md` for references to other table files (e.g., links like `[orders](orders.md)` or `[users](users.md)`).
  - Use these links to build a relationship graph of the database.

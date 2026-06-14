# `okf-go` API reference (for producer authors)

`okf-go` is the shared library every skill in this project imports. **It is the
only place OKF types live** — never redefine `Frontmatter` or `ConceptDoc` in a
skill. Using these helpers is what makes a producer's output spec-conformant and
round-trippable by `ingest` and `okf-enrich`.

## Import & module wiring

```go
import "github.com/savikne/okf-skills/okf-go" // package name: okf
```

In the skill's `go.mod` (copy the toolchain line and the `replace` from an
existing skill such as `skills/okf-sqlite/go.mod`):

```go
module okf-<name>

go 1.24.0

require github.com/savikne/okf-skills/okf-go v0.0.0

// Local path mapping so the skill builds in a cloned/sandboxed checkout.
replace github.com/savikne/okf-skills/okf-go => ../../okf-go
```

---

## Concept documents

### `type Frontmatter`

The YAML metadata block at the top of every concept document. All fields are
`omitempty`, so zero values are simply absent from the output.

| Field | YAML key | Notes |
|---|---|---|
| `Type` | `type` | **The one required OKF field.** A short concept-kind string, e.g. `"SQLite Table"`, `"MongoDB Collection"`. |
| `Title` | `title` | Human display name (usually the asset name). |
| `Description` | `description` | One-line summary. **Producers write a deterministic placeholder here; `okf-enrich` rewrites it.** |
| `Resource` | `resource` | Canonical URI of the underlying asset. **Never embed credentials.** |
| `Tags` | `tags` | `[]string` for cross-cutting categorization. |
| `Timestamp` | `timestamp` | ISO-8601 / RFC3339 last-modified time. Compute once with `time.Now().Format(time.RFC3339)`. |
| `OKFVersion` | `okf_version` | **Only permitted in the bundle-root `index.md`.** Set to `"0.1"` there and leave every other field empty. |

### `type ConceptDoc`

```go
type ConceptDoc struct {
    Frontmatter Frontmatter
    Body        string // raw markdown after the closing `---`
}
```

### `WriteConceptDoc(filePath string, doc ConceptDoc) error`

Serializes a `ConceptDoc` to `filePath` (`---\n` + YAML + `---\n` + body, mode
`0644`). Create the parent directory yourself with `os.MkdirAll` first.

### `ReadConceptDoc(filePath string) (*ConceptDoc, error)`

Parses a concept file back into a `ConceptDoc`. Handles both LF and CRLF
frontmatter boundaries. Use this in `ingest`. Returns an error on a file that
has no `---`-delimited frontmatter.

---

## Body sections

The body is free-form markdown, but producers emit a `# Columns` table for the
asset's schema and may append `## Data Profile` / `## Sample` sections. These
helpers manage those sections without clobbering surrounding content.

### `UpsertSection(body, heading, content string) string`

Inserts or replaces a level-2 (`## heading`) section. If the heading exists, its
content (up to the next `#`/`##` heading) is replaced; otherwise the section is
appended. Use it to attach optional sections:

```go
bodyStr = okf.UpsertSection(bodyStr, "Data Profile", okf.RenderProfileSection(profiles))
bodyStr = okf.UpsertSection(bodyStr, "Sample", okf.RenderSampleSection(headers, rows))
```

### `GetSection(body, heading string) (string, bool)`

Returns the content beneath a **level-2** `## heading`, trimmed, and whether it
was found.

### `GetSectionAny(body, heading string) (string, bool)`

Like `GetSection`, but matches the heading at **any** ATX level — including the
level-1 `# Columns` heading producers emit for the schema table. **Use this in
`ingest` to isolate the schema table before parsing rows**, so appended
`## Data Profile` / `## Sample` rows are never mistaken for schema columns:

```go
body := doc.Body
if section, ok := okf.GetSectionAny(doc.Body, "Columns"); ok {
    body = section // parse columns only from here
}
```

---

## Profile & sample rendering

Used by the optional `--profile` / `--sample` flags. Build the data with your
source's queries, then render to markdown tables.

### `type ColumnProfile`

```go
type ColumnProfile struct {
    Column   string // column / field name
    NonNull  int64
    Null     int64
    Distinct int64
    Min      string // rendered as text
    Max      string // rendered as text
}
```

### `RenderProfileSection(profiles []ColumnProfile) string`

Renders `| Column | Non-Null | Null | Distinct | Min | Max |`. Feed the result
to `UpsertSection(body, "Data Profile", …)`.

### `RenderSampleSection(headers []string, rows [][]string) string`

Renders a sample-rows table from headers + row cells. Feed to
`UpsertSection(body, "Sample", …)`.

### `SanitizeCell(s string) string`

Makes any value safe for a single markdown table cell (escapes `|`, flattens
newlines, trims). **Call it on every cell you write into a table** — including
the schema table you build by hand.

---

## File-like sources: ignore rules & sidecar metadata

For filesystem/VCS-style producers (`okf-fs`, `okf-git`).

### `NewIgnoreMatcher(root string) (*IgnoreMatcher, error)` / `(*IgnoreMatcher).Matches(relPath string) bool`

Loads `.okfignore` (gitignore-ish wildcards) from `root`; `Matches` reports
whether a bundle-relative path should be skipped. `.git/` and `.okfignore`
itself are always ignored.

### `ReadFolderMetadata(dirPath string) (map[string]string, error)` / `WriteFolderMetadata(dirPath string, meta map[string]string) error`

Read/write `.okf-metadata.yaml` (a `path → description` map). **This is the
sync target for sources with no native comment store** — `ingest --sync` writes
descriptions here for `okf-fs`/`okf-git`. Writing an empty map removes the file.

---

## The `schema` self-description (MCP contract)

`okf-mcp` discovers any `okf-*` binary that answers `schema` and exposes its
`produce`/`ingest` commands as MCP tools. Implementing `schema` is the **only**
thing required for auto-discovery — no `okf-mcp` changes.

### `type FlagSchema`

```go
type FlagSchema struct {
    Name        string // flag name without dashes, e.g. "db"
    Type        string // "string" | "int" | "bool"
    Description string
    Required    bool
    Default     string // rendered as text, optional
    Env         string // env var that may supply this value, optional
}
```

**`Env` is how secrets stay out of argv.** Declare it on any flag that carries a
password / token / connection URI; `okf-mcp` then passes that value through the
child process environment instead of the command line. Resolve the same env var
in your command code as a fallback when the flag is empty.

### `type CommandSchema` / `type SkillSchema`

```go
type CommandSchema struct { Name, Description string; Flags []FlagSchema }
type SkillSchema    struct { Name, Description string; Commands []CommandSchema }
```

`SkillSchema.Name` MUST equal the binary/skill name (`okf-<name>`). Always
declare all three commands: `produce`, `ingest`, `schema`.

### `PrintSchema(w io.Writer, s SkillSchema) error`

Writes the schema as indented JSON. Wire it into the `schema` case of `main`:

```go
case "schema":
    if err := okf.PrintSchema(os.Stdout, buildSchema()); err != nil {
        log.Fatalf("Failed to print schema: %v", err)
    }
```

---

## Quick map: helper → when you reach for it

| Goal | Helper |
|---|---|
| Write a concept file | `WriteConceptDoc` |
| Read a concept file (in `ingest`) | `ReadConceptDoc` |
| Isolate the `# Columns` table before parsing | `GetSectionAny(body, "Columns")` |
| Attach an optional `## Data Profile` / `## Sample` | `UpsertSection` + `RenderProfileSection` / `RenderSampleSection` |
| Make a cell safe | `SanitizeCell` |
| Skip ignored paths (fs/git) | `NewIgnoreMatcher` + `Matches` |
| Sync descriptions for a comment-less, file-like source | `ReadFolderMetadata` / `WriteFolderMetadata` |
| Self-describe for `okf-mcp` | `SkillSchema` + `PrintSchema` |

# `okf-go` API reference (for producer authors)

`okf-go` is the shared library every skill in this project imports. **It is the
only place OKF types live** — never redefine `Frontmatter` or `ConceptDoc` in a
skill. Using these helpers is what makes a producer's output spec-conformant and
round-trippable by `ingest` and `okf-enrich`.

## Import & module wiring

```go
import "github.com/xSAVIKx/okf-skills/okf-go" // package name: okf
```

In the skill's `go.mod` (copy the toolchain line from an existing skill such as
`skills/okf-sqlite/go.mod`):

```go
module github.com/xSAVIKx/okf-skills/skills/okf-<name>

go 1.24.0

require github.com/xSAVIKx/okf-skills/okf-go v0.1.0
```

Use the **full, publishable module path** (not a bare `okf-<name>`) so the skill
can be `go install`ed standalone. Do **not** add a `replace` directive: the root
`go.work` maps `okf-go` to the on-disk copy for local development, and the clean
`go.mod` resolves the published `okf-go` when the skill is installed on its own.
After creating the module, add it to `go.work` (`go work use ./skills/okf-<name>`).

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
to `UpsertSection(body, "Data Profile", …)`. When any profile carries a
`Semantic` tag, a `Semantic` column is added and low-cardinality columns get a
`col ∈ {…}` line beneath the table; otherwise the legacy six-column table renders
unchanged (byte-stable for semantic-free bundles).

### Semantic grounding: `DetectSemanticType` / `ClassifyColumn`

`ColumnProfile` carries `Semantic string` and `Values []string`. Populate them
from values you already read — no second scan: pull up to `okf.LowCardinalityN+1`
(=13) distinct values per column (`SELECT DISTINCT col … LIMIT 13`) and call:

```go
semantic, values := okf.ClassifyColumn(col.Name, distinctVals, distinct)
prof.Semantic, prof.Values = semantic, values
```

`ClassifyColumn` layers the connector's structural knowledge (column name, distinct
count) onto the pure value verdict of `DetectSemanticType(samples) string`
(`email`/`uuid`/`iso-timestamp`/`monetary`/`boolean` by ≥90% majority match, else
`""`): it adds `enum` for `0 < distinct ≤ N`, advisory `fk-ish` for an id-shaped
name with all-integer values, and returns the sorted distinct set for
low-cardinality columns. All deterministic, no LLM.

### `RenderSampleSection(headers []string, rows [][]string) string`

Renders a sample-rows table from headers + row cells. Feed to
`UpsertSection(body, "Sample", …)`.

### `SanitizeCell(s string) string`

Makes any value safe for a single markdown table cell (escapes `|`, flattens
newlines, trims). **Call it on every cell you write into a table** — including
the schema table you build by hand.

### SQL metadata: constraints, indexes, stats, views

For SQL sources, surface the cheap structural signals beyond columns. Types live
in `okf` (`Constraint{Name,Type,Definition}`, `Index{Name,Columns,Unique}`,
`TableStats{RowCount,HasRowCount,FreshnessColumn,Earliest,Latest}`); each renderer
returns `""` when there is nothing to show, so guard the `UpsertSection`:

```go
if s := okf.RenderConstraintsSection(cons); s != "" { body = okf.UpsertSection(body, "Constraints", s) }
if s := okf.RenderIndexesSection(idx);     s != "" { body = okf.UpsertSection(body, "Indexes", s) }
if isView { if s := okf.RenderViewDefinition(viewSQL); s != "" { body = okf.UpsertSection(body, "View Definition", s) } }
if *stats { if s := okf.RenderStatsSection(ts); s != "" { body = okf.UpsertSection(body, "Stats", s) } }
```

Conventions: distinguish a view in the concept `type` (e.g. `"PostgreSQL View"`
vs `"PostgreSQL Table"`) and capture its defining SQL in `## View Definition`.
Constraints/indexes are cheap catalog reads (emit by default); row-count and
freshness are heavier, so gate them behind a `--stats` flag.

---

## Relationships (typed cross-links)

Used by the optional `--relationships` flag. Extract structural edges your source
already declares — foreign keys, file co-change pairs — **deterministically**, and
emit them as ordinary bundle-relative markdown links. `okf-viz` picks them up as
graph edges with no changes, and `okf-enrich` later explains what each edge
*means* in prose. **Never embed an LLM to build a relationship** — the connector
emits the edge as a fact; meaning is added downstream.

### `type Relationship`

```go
type Relationship struct {
    Label  string // e.g. "FK on customer_id", "co-changed (42x)"
    Target string // bundle-relative link target, e.g. "/tables/customers.md"
    Text   string // link text shown to the reader, e.g. "customers"
}
```

### `RenderRelationshipsSection(rels []Relationship) string`

Renders a markdown bullet list of `- <Label> [<Text>](<Target>)` links, **sorted
by `(Target, Label, Text)`** so re-runs over the same input are byte-identical.

### `AppendRelationshipsSection(body, heading string, rels []Relationship) string`

Appends a level-1 `# <heading>` section (e.g. `# Relationships` for SQL FKs,
`# Related Files` for git co-change) containing the rendered links. **Returns
`body` unchanged when `rels` is empty** — a source with no relationships emits no
section rather than an empty one. The level-1 heading mirrors `# Columns`, so both
`GetSectionAny` and the structural hash pick it up.

```go
bodyStr = okf.AppendRelationshipsSection(bodyStr, "Relationships", rels)
```

---

## Incremental produce (don't clobber enrichment)

Every `produce` is a full re-extraction. Without care it overwrites the enriched
`description` an agent wrote and churns git diffs on unchanged concepts. Use these
helpers in your produce loop so a re-run preserves unchanged concepts **byte-for-byte**
and rewrites only what structurally changed.

### `Frontmatter.ContentHash` (`content_hash`)

A per-concept structural hash, stamped by `MergeConcept`. Excluded from the hash
itself; set it via the merge helpers, don't compute it by hand.

### `ConceptStructuralHash(doc ConceptDoc) string`

SHA-256 over the concept **body** (line-endings normalized), deliberately ignoring
`Description`/`Timestamp`/`ContentHash` (all frontmatter). A description- or
timestamp-only change yields the **same** hash; a column/profile/relationship
change yields a different one.

### `MergeConcept(existing *ConceptDoc, fresh ConceptDoc) (ConceptDoc, bool)`

The core of the loop. Returns `(docToWrite, changed)`:

- `existing == nil` → returns `fresh` with `ContentHash` stamped, `changed == true`.
- structure unchanged → returns the existing doc, `changed == false`; **skip the write**.
- structure changed → returns a merged doc (fresh body + new hash/timestamp) that
  carries over the agent's `Description`, the union of `Tags`, and any agent-added
  body sections; `changed == true`.

```go
fresh := okf.ConceptDoc{ /* …deterministic extraction… */ }
var existing *okf.ConceptDoc
if e, err := okf.ReadConceptDoc(filePath); err == nil { existing = e }
merged, changed := okf.MergeConcept(existing, fresh)
if !changed { continue }                 // preserved byte-for-byte
okf.WriteConceptDoc(filePath, merged)
kind, action := "Update", "Structure changed for"
if existing == nil { kind, action = "Creation", "Established" }
okf.AppendLogEntry(outDir, today, kind, fmt.Sprintf("%s [%s](%s).", action, title, bundlePath))
```

### `AppendLogEntry(bundleDir, date, kind, message string) error`

Appends an OKF-SPEC §7 entry to `<bundle>/log.md`: newest-first `## YYYY-MM-DD`
date headings with `* **<Kind>**: <message>` bullets. Pass `date` =
`time.Now().Format("2006-01-02")`.

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
| Emit deterministic typed edges (FKs, co-change) | `AppendRelationshipsSection` + `RenderRelationshipsSection` |
| Preserve enrichment across re-produce | `MergeConcept` (+ `ConceptStructuralHash`) |
| Record a change to the bundle | `AppendLogEntry` |
| Make a cell safe | `SanitizeCell` |
| Skip ignored paths (fs/git) | `NewIgnoreMatcher` + `Matches` |
| Sync descriptions for a comment-less, file-like source | `ReadFolderMetadata` / `WriteFolderMetadata` |
| Self-describe for `okf-mcp` | `SkillSchema` + `PrintSchema` |

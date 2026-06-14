# Releasing

Releases are fully automated from [Conventional Commits](https://www.conventionalcommits.org/)
via [release-please](https://github.com/googleapis/release-please). You never tag
or publish a Go module by hand.

## The pipeline

| Workflow | Trigger | What it does |
|---|---|---|
| [`ci.yml`](.github/workflows/ci.yml) | PRs + pushes to `master` | gofmt, `go vet`, build every module, unit tests, and the Docker-backed integration suite. |
| [`pr-title.yml`](.github/workflows/pr-title.yml) | PR opened/edited | Lints the PR title against Conventional Commits (it becomes the squash-merge commit). |
| [`release.yml`](.github/workflows/release.yml) | pushes to `master` | release-please maintains a release PR; merging it tags + releases each changed module, then warms the Go proxy. |

## How a release happens

1. **Land work with conventional commit messages.** Use squash-merge so the PR
   title is the commit on `master`. The type drives the bump (pre-1.0 semantics,
   `bump-minor-pre-major`):
   - `fix:` → patch (`0.1.0` → `0.1.1`)
   - `feat:` → minor (`0.1.0` → `0.2.0`)
   - `feat!:` / `BREAKING CHANGE:` → minor while `0.x` (won't jump to `1.0.0`)
   - `chore/docs/ci/test/style/build/refactor:` → no release on their own
2. **Scope the commit to a module** so release-please bumps the right one:
   `fix(okf-sqlite): ...`, `feat(okf-go): ...`. The scope is informational;
   release-please actually decides which modules to bump from the **file paths**
   touched by the commits, mapped via `packages` in `release-please-config.json`.
3. **release-please opens a "release PR"** titled like `chore: release main`,
   updating `.release-please-manifest.json`, each changed module's version, and
   its `CHANGELOG.md`. Review it like any PR.
4. **Merge the release PR.** release-please then, per changed module:
   - creates the git tag in Go's required form — `okf-go/v0.2.0`,
     `skills/okf-fs/v0.2.0`, … (this *is* publishing the module), and
   - creates a GitHub Release with the changelog.
5. **`warm-proxy`** requests each new version from `proxy.golang.org` so
   `go install github.com/xSAVIKx/okf-skills/skills/<name>@latest` resolves right
   away instead of after the mirror's indexing lag.

## Tag format

Go's module proxy requires a submodule's tag to be `<repo-relative-path>/vX.Y.Z`.
That is produced by these `release-please-config.json` settings:

```jsonc
"include-component-in-tag": true,   // prepend the component
"include-v-in-tag": true,           // ...and a "v"
"tag-separator": "/",               // joined with "/"
"packages": { "skills/okf-fs": { "component": "skills/okf-fs" } }  // component = full path
// => tag: skills/okf-fs/v<version>
```

## Cross-module dependency note

The skills require `github.com/xSAVIKx/okf-skills/okf-go` at a pinned version.
release-please does **not** auto-rewrite that `require` line when `okf-go` is
released. When you ship an `okf-go` change that skills should adopt, bump the
requirement in the same PR with a `deps:` commit, e.g.
`deps(okf-sqlite): use okf-go v0.2.0`, so the skill re-releases against it.

## One-time repository setup

- **Settings → Actions → General → Workflow permissions:** enable
  *Read and write permissions* and *Allow GitHub Actions to create and approve
  pull requests* (release-please opens the release PR with `GITHUB_TOKEN`).
- **Settings → General → Pull Requests:** keep *Allow squash merging* enabled
  (and ideally make it the default) so PR titles flow into `master` as
  conventional commits.

# Releasing

Releases are fully automated from [Conventional Commits](https://www.conventionalcommits.org/)
via [release-please](https://github.com/googleapis/release-please). You never tag
or publish a Go module by hand.

## The pipeline

| Workflow | Trigger | What it does |
|---|---|---|
| [`ci.yml`](.github/workflows/ci.yml) | PRs + pushes to `master` | gofmt, `go vet`, build every module, unit tests, and the Docker-backed integration suite. On the release PR it first runs [`scripts/ci-localize-okfgo.sh`](scripts/ci-localize-okfgo.sh) so the bumped-but-unpublished `okf-go` pin resolves (no-op elsewhere). |
| [`pr-title.yml`](.github/workflows/pr-title.yml) | PR opened/edited | Lints the PR title against Conventional Commits (it becomes the squash-merge commit). |
| [`release.yml`](.github/workflows/release.yml) | pushes to `master` | release-please maintains a release PR; merging it tags + releases each changed module, warms the Go proxy, then `verify-install` confirms every released module installs standalone. |

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

## Cross-module dependency: the `okf-go` pin (lockstep)

Versioning is **lockstep**: every release bumps all modules to the same version
and tags them at the same commit. So `skills/okf-sqlite@v0.2.0` requires
`okf-go@v0.2.0` and both exist at the same SHA — no skew between a connector and
the okf-go it was built against. This is enforced by the **`linked-versions`**
plugin in [`release-please-config.json`](release-please-config.json), which bumps
every listed component to the same version when any one of them changes (so even
modules untouched by a release still re-release to carry the new okf-go pin).

Two facts make this non-trivial, and both are **hidden by [`go.work`](go.work)**.
During development the workspace makes every `require okf-go vX` resolve to
working-tree source, ignoring the pin, and `go.sum` is not consulted for
workspace modules. As a result a normal `go build`/`go test` in CI runs *inside*
the workspace and **cannot** catch:

1. a stale **pin** — `require okf-go vX` drifting from the version the code needs
   (e.g. a connector using new okf-go API while still pinned to the old release);
2. a missing **`go.sum`** entry for the okf-go version a real install resolves.

Both only surface in an end user's `go install <module>@vX`.

### Safety net: the `verify-install` gate

After tags are pushed, the `verify-install` job runs
[`scripts/verify-release.sh`](scripts/verify-release.sh), which `go install`s
each released module with `GOWORK=off` (no workspace), `GOPROXY=direct` (no proxy
indexing lag), and `GOSUMDB=off` (the public checksum DB lags new tags) while
still verifying each module's committed `go.sum`. A stale pin or missing `go.sum`
fails the release loudly instead of failing a user. The gate is **post-release,
not per-PR** on purpose: a PR that adds okf-go API and consumes it in the same
change cannot build standalone until okf-go is released, so gating PRs on it
would block the normal stacked-PR workflow.

### Bumping the pin + `go.sum` — automated (`sync-pins`)

There is a chicken-and-egg: a connector's `go.sum` needs `okf-go@vNEW`'s hash,
but that hash is only resolvable once `okf-go@vNEW` is tagged — the very release
being built. The escape: Go module hashes are **content-based, not SHA-based**,
so a `go.sum` computed against a *locally* created tag on the release-PR branch
is identical to the one the real pushed tag yields.

The **`sync-pins`** job in [`release.yml`](.github/workflows/release.yml) does
this automatically. When release-please opens/updates the release PR, the job
checks out that branch and runs
[`scripts/sync-intra-deps.sh`](scripts/sync-intra-deps.sh), which:

1. reads the new lockstep version from `.release-please-manifest.json`;
2. creates the `okf-go/vNEW` tag **locally** (not pushed — release-please tags
   the same commit on merge);
3. sets `GOPRIVATE=github.com/xSAVIKx/okf-skills` and
   `git config url."file://<repo>".insteadOf https://github.com/xSAVIKx/okf-skills`
   so `go mod tidy` resolves the unpushed tag from the working tree;
4. rewrites every consumer's `require okf-go` to `vNEW` and `go mod tidy`s each;
5. **builds every consumer standalone** (`GOWORK=off`) against the local tag, so
   the run fails here if the synced pins/`go.sum` don't actually compile.

then commits the `go.mod`/`go.sum` changes back to the release PR. It is
idempotent (pins already at `vNEW` → no commit). On merge, the tagged commit is
internally consistent, and `verify-install` confirms it against the real tags.

> **Why the release PR needs its own verification.** Step 5 above is the real
> pre-merge check on the *standalone* (no-`go.work`) build: it runs in the
> `sync-pins` job and verifies the exact synced state, and `verify-install` is
> the post-merge backstop against the published tags. The full `ci.yml` suite
> (gofmt, vet, unit + integration tests) *does* run on the release PR — the
> `sync-pins` push triggers a `pull_request` event — but `ci.yml` builds *inside*
> `go.work`, where the bumped `require okf-go vNEW` would otherwise fail to
> resolve because `okf-go/vNEW` is not tagged until merge. Even in workspace mode
> Go still selects `okf-go@vNEW` during module-graph resolution and reads its
> `go.mod` from the tag. So each `ci.yml` job first runs
> [`scripts/ci-localize-okfgo.sh`](scripts/ci-localize-okfgo.sh), which (only when
> `okf-go/vNEW` is unpublished) creates that tag locally at `HEAD` and points
> `GOPRIVATE` + `git insteadOf` at this checkout — the same resolution
> `sync-intra-deps.sh` uses. The build then proceeds in `go.work` as normal. On
> any other branch the pinned version is already published, so the script is a
> no-op.

> Validated on Linux via [`scripts/dryrun-pin-sync.sh`](scripts/dryrun-pin-sync.sh)
> (run in a `golang` container): all consumers re-pin, `go.sum` refreshes, and
> every module builds standalone against the unpushed tag.

Fallbacks if you ever disable `sync-pins`: do step 4 by hand on the release PR
branch (manual), or release `okf-go` first and bump consumers the next cycle
(two-phase, but consumers then lag okf-go by one release).

> Do **not** bump `require okf-go vNEW` in a *feature* PR: `vNEW` does not exist
> until the release PR merges, so the pin would reference an unpublished version.
> The bump belongs in the release PR, which `sync-pins` handles.

## One-time repository setup

- **Settings → Actions → General → Workflow permissions:** enable
  *Read and write permissions* and *Allow GitHub Actions to create and approve
  pull requests* (release-please opens the release PR with `GITHUB_TOKEN`).
- **Settings → General → Pull Requests:** keep *Allow squash merging* enabled
  (and ideally make it the default) so PR titles flow into `master` as
  conventional commits.

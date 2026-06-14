# Contributing

All changes land through **pull requests** — `master` is never pushed to directly.

## Workflow

1. **Branch** off `master` with a descriptive name:
   ```bash
   git switch -c feat/sqlite-views   # or fix/..., docs/..., ci/..., refactor/...
   ```
2. **Commit** using [Conventional Commits](https://www.conventionalcommits.org/).
   Scope to the module you touched so release-please bumps the right one:
   ```
   feat(okf-sqlite): extract view definitions
   fix(okf-go): handle empty frontmatter
   ```
3. **Open a PR.** CI ([`.github/workflows/ci.yml`](.github/workflows/ci.yml)) runs
   gofmt, `go vet`, builds every module, unit tests, and the Docker-backed
   integration suite. The PR title is linted against Conventional Commits.
4. **Squash-merge** once CI is green. The squash commit message is the PR title,
   so it must be a valid conventional commit — that is what drives releases.

## What happens after merge

release-please maintains a release PR that bumps versions and changelogs;
merging it tags and publishes each changed Go module and cuts a GitHub Release.
See **[RELEASING.md](RELEASING.md)** for the details.

## Local checks before pushing

```bash
gofmt -l $(git ls-files '*.go')   # should print nothing
make build                        # build all modules
make test                         # unit tests
cd tests && go test ./...         # integration (MySQL/PostgreSQL cases need docker-compose up)
```

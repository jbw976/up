# AGENTS.md

Guidance for AI coding assistants (Claude Code, GitHub Copilot, Cursor,
etc.) working in this repository. User-facing docs live in
[`README.md`](README.md) and [`CONTRIBUTING.md`](CONTRIBUTING.md) —
CONTRIBUTING covers kong conventions, embedded-docs rendering rules,
telemetry, Windows path handling, and the release playbook in detail,
and is the authoritative source for those topics.

> This file is the source of truth. `CLAUDE.md` and
> `.github/copilot-instructions.md` are symlinks to it — edit this one.

## What's in this repo

`up` is the Upbound CLI plus the `docker-credential-up` credential
helper, built from one Go module rooted at `github.com/upbound/up`.

- [`cmd/up/`](cmd/up/) — the CLI. One directory per top-level verb
  (`project/`, `ctx/`, `dependency/`, `xpkg/`, `uxp/`, …). The CLI is
  [kong](https://pkg.go.dev/github.com/alecthomas/kong)-driven; command
  structs are the source of truth for flags, args, help, and docs.
- [`cmd/docker-credential-up/`](cmd/docker-credential-up/),
  [`cmd/schema-generator/`](cmd/schema-generator/) — auxiliary binaries.
- [`pkg/`](pkg/) — public, importable packages (the API surface of the
  module).
- [`internal/`](internal/) — everything not meant to be importable.
  Expect to spend most of your time here.
- [`e2e/`](e2e/) — not Go tests. Golden `.yaml`/`.k` files that
  `.github/workflows/_e2e.yml` asserts against via
  [assert-command-line-output](https://github.com/GuillaumeFalourd/assert-command-line-output).
  To add an e2e case, edit the workflow and drop the expected output in
  `e2e/`.

### Things with separate rules

- **[`pkg/migration/`](pkg/migration/) is its own Go module** (separate
  `go.mod`). Treat it as a sibling module: `go mod tidy` there
  independently, and mind its own dep graph when bumping versions.
- **[`internal/vendor/`](internal/vendor/) is hand-vendored**, not
  `go mod vendor`. It re-exposes packages that upstream hides behind
  `internal/` (from `golang.org/x/tools` and `crossplane/crossplane`).
  See [`internal/vendor/README.md`](internal/vendor/README.md). Don't
  edit it casually; refresh by re-copying from upstream at a known
  version.

## Before pushing — mirror what CI runs

`.github/workflows/ci.yml` gates PRs on `lint`, `check-diff`,
`unit-tests`, `build`, and `e2e-tests`. Locally:

```bash
make reviewable   # lint + test + integration-test
make check-diff   # generate + go mod tidy, fail if the tree is dirty
```

Notes:

- CI's `unit-tests` job actually runs `make integration-test`
  (`go test -tags=integration ./...`) — the plain `-tags` build is what
  gates PRs. `make reviewable` runs both, which is what you want.
- `make lint` uses `--fix --new-from-merge-base=origin/main`, so CI only
  flags *new* issues relative to `main`. Keep that scope in mind when
  touching older files — see the CONTRIBUTING note about using `nolint`
  judiciously and fixing issues in files you touch.
- golangci-lint version is pinned in `.github/workflows/ci.yml`
  (`golangci-version` input to `_check.yml`). Match it locally.

## Running locally

- `make build` — goreleaser snapshot for the current platform (into
  `_output/`). `make build.all` for all platforms.
- `make generate` — `go generate ./...` (license headers, deepcopies).
- Run a single test: `go test ./path/to/pkg -run TestName -v`. Use
  `-tags=integration` to include the integration-tagged tests. There's
  no `TEST_NAME`/`RUN` make variable — go directly.

## Go conventions in this codebase

- **Test assertions**: prefer
  [`gotest.tools/v3/assert`](https://pkg.go.dev/gotest.tools/v3/assert)
  over `testify`. Plain `testing` + `google/go-cmp` is also fine. Match
  the nearest neighbor file.
- **CLI commands are kong structs.** Flags, help, and auto-completion
  flow from struct tags. Long-form help goes in a command's `Help()`
  method and is embedded into
  [docs.upbound.io](https://docs.upbound.io/reference/cli/) — see
  CONTRIBUTING for heading-level and `<` escaping rules before writing
  help.
- **Telemetry**: mark non-sensitive flags with `telemetry:"true"` struct
  tags. Never put identifying or sensitive values on spans. Details in
  CONTRIBUTING.
- **Windows paths**: `path` for in-memory `afero` FSes, `filepath` for
  OS paths, and prefer `internal/filesystem.Walk` over `afero.Walk`.
  CONTRIBUTING has the full rubric — read it before touching filesystem
  code.
- **Generated code**: regenerate via `make generate`. Don't hand-edit
  `zz_generated_*.go`.

## Release and branch model

- `main` is latest. Released lines live on `release-X.Y` branches (first
  release of a new minor creates the branch via the GitHub UI).
- Release is semi-automated: run the `Release` GH Action against the
  release branch, then `Promote` against the tag. CONTRIBUTING's
  "Release Process" section is the playbook; follow it end-to-end.
- Homebrew, docs PR, CDN cache, and announcement steps are part of that
  playbook — don't skip them.

## Commits and PRs

- Sign off every commit (`git commit --signoff`).
- Semantic-commit style (renovate uses `:semanticCommits`). Recent
  history: `fix(project-ai): …`, `feat(…): …`, `chore(deps): …`.
- Renovate owns dep bumps; avoid manual upgrades that would conflict.
- To backport, label the PR `backport release-X.Y` **before it merges**
  — `.github/workflows/backport.yml` opens the backport PR on merge
  (merge-commit strategy required; the action doesn't support squash).
  Or comment `/backport release-X.Y` after merge (see `commands.yml`).
- There is no `.github/PULL_REQUEST_TEMPLATE.md` in this repo. Write a
  short Overview + Validation section in the PR body; reference issue(s)
  with `Fixes #`.

## Keeping this file useful

If you hit a gotcha or invariant that will bite the next agent, capture
it here — but only if it has wider value, not one-off fixes already
encoded in a PR. Keep the root file tight: once a note grows past a few
lines (runbook, deep-dive, domain workflow), move it to its own file
under `design/` or alongside the relevant package, and link to it from
here.

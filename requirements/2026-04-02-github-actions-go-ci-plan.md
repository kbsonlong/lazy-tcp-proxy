# GitHub Actions Go CI Workflow — Implementation Plan

**Requirement**: [2026-04-02-github-actions-go-ci.md](2026-04-02-github-actions-go-ci.md)
**Date**: 2026-04-02
**Status**: Implemented

## Implementation Steps

1. Create `.github/workflows/go-ci.yml` with the workflow definition below.
2. Update `requirements/2026-04-02-github-actions-go-ci.md` status → Completed.
3. Update `requirements/_index.md` REQ-031 status → Completed.
4. Commit and push all changes.

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `.github/workflows/go-ci.yml` | Create | CI workflow: vet, build, test, lint |
| `requirements/2026-04-02-github-actions-go-ci.md` | Modify | Status → Completed |
| `requirements/_index.md` | Modify | REQ-031 status → Completed |

## Workflow Design

### Triggers

```yaml
on:
  push:
    branches: ["**"]
  pull_request:
    branches: ["**"]
```

Runs on every push and every PR across all branches within the repo. Forks do not share secrets but the `pull_request` trigger still fires for in-repo PRs; fork PRs are outside scope per requirement.

### Jobs

Two jobs run in parallel:

**`lint`** — fast static analysis, fails early on obvious issues  
**`test`** — build verification + full test suite with race detector

Both use:
- `runs-on: ubuntu-latest`
- `actions/checkout@v4`
- `actions/setup-go@v5` with `go-version-file: lazy-tcp-proxy/go.mod` (auto-reads `go 1.24` from the module file)
- Module cache via `actions/setup-go@v5`'s built-in `cache: true` + `cache-dependency-path: lazy-tcp-proxy/go.sum`

### `lint` job steps

1. `actions/checkout@v4`
2. `actions/setup-go@v5` — reads go version from `go.mod`, caches modules
3. `golangci/golangci-lint-action@v8` — runs in `./lazy-tcp-proxy`; no `go.mod` changes

### `test` job steps

1. `actions/checkout@v4`
2. `actions/setup-go@v5`
3. `go vet ./...` (working-directory: `./lazy-tcp-proxy`)
4. `go build ./...` (working-directory: `./lazy-tcp-proxy`)
5. `go test -race -count=1 ./...` (working-directory: `./lazy-tcp-proxy`)

### Full workflow YAML

```yaml
name: Go CI

on:
  push:
    branches: ["**"]
  pull_request:
    branches: ["**"]

jobs:
  lint:
    name: Lint
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: lazy-tcp-proxy/go.mod
          cache-dependency-path: lazy-tcp-proxy/go.sum
      - uses: golangci/golangci-lint-action@v8
        with:
          working-directory: lazy-tcp-proxy

  test:
    name: Test & Build
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: lazy-tcp-proxy/go.mod
          cache-dependency-path: lazy-tcp-proxy/go.sum
      - name: vet
        working-directory: lazy-tcp-proxy
        run: go vet ./...
      - name: build
        working-directory: lazy-tcp-proxy
        run: go build ./...
      - name: test
        working-directory: lazy-tcp-proxy
        run: go test -race -count=1 ./...
```

## Risks & Open Questions

- `golangci-lint-action@v8` is the current major version at time of writing; Dependabot will keep it updated.
- `-race` requires CGO; `ubuntu-latest` includes all required toolchain components so this is safe.
- No secrets are needed by either job.

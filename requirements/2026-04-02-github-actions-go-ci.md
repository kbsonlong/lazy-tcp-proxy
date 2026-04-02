# GitHub Actions Go CI Workflow

**Date Added**: 2026-04-02
**Priority**: High
**Status**: In Progress

## Problem Statement

There is no automated CI pipeline for the Go codebase in `lazy-tcp-proxy/`. Pull requests and pushes to `main` are not automatically validated, meaning broken builds and failing tests can land undetected.

## Functional Requirements

1. On every push to `main` and every pull request targeting `main`, automatically run the Go build and test suite.
2. The workflow MUST fail if `go build` fails.
3. The workflow MUST fail if any `go test` fails.
4. The workflow SHOULD also run `go vet` to catch common static-analysis issues.
5. The workflow SHOULD run `golangci-lint` for broader lint coverage (staticcheck, errcheck, etc.).
6. The Go module cache SHOULD be cached between runs to speed up CI.

## User Experience Requirements

- Developers get fast, clear feedback on PRs — separate jobs for lint vs test/build make failure reasons obvious.
- Dependabot can keep the `github-actions` action versions up to date (already configured in `.github/dependabot.yml`).

## Technical Requirements

- Working directory for all Go commands: `./lazy-tcp-proxy`
- Go version: `1.24` (matches `go.mod`)
- Runner: `ubuntu-latest` (Docker-specific code; linux-only is sufficient)
- Use pinned, tagged action versions so Dependabot can raise PRs for updates
- `go test` must include the `-race` flag to catch data races
- `go test` must include the `-count=1` flag to disable test caching (ensures a clean run)
- Integration tests in `internal/proxy/integration_test.go` use an in-process mock — no real Docker daemon is required; `go test ./...` runs all tests including these

## Acceptance Criteria

- [ ] Workflow file exists at `.github/workflows/go-ci.yml`
- [ ] Workflow triggers on `push` to `main` and `pull_request` targeting `main`
- [ ] `go vet ./...` step passes on the main branch
- [ ] `go build ./...` step passes on the main branch
- [ ] `go test -race -count=1 ./...` step passes on the main branch
- [ ] `golangci-lint` step passes on the main branch
- [ ] Go module cache is restored/saved between runs
- [ ] Dependabot can manage `github-actions` dependency updates (already configured)

## Dependencies

- Depends on the existing Go module at `lazy-tcp-proxy/go.mod`
- No other requirements are blocked by this one
- Enables faster feedback loop for all future requirement implementations

## Implementation Notes

- `golangci-lint` will be added via the official `golangci/golangci-lint-action` — no need to install it manually
- Integration tests (`integration_test.go`) use a build tag; the default `go test ./...` will skip them unless `-tags integration` is passed — confirm this is the case before finalising the plan
- The workflow should NOT require Docker-in-Docker; unit tests should run without Docker

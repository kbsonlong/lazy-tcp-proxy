# Fix golangci-lint errcheck Violations

**Date Added**: 2026-04-02
**Priority**: High
**Status**: Completed

## Problem Statement

The new Go CI workflow (REQ-031) runs `golangci-lint`, which reported 10 `errcheck` violations — unchecked error return values from `Close()` calls across `server.go`, `integration_test.go`, and `manager.go`.

## Functional Requirements

All `errcheck` lint violations must be resolved so the CI lint job passes.

## Technical Requirements

Two categories of fix apply:

**A — Cleanup `defer` / test teardown calls** (errors are not actionable; suppress with `//nolint:errcheck`):
- `server.go:169` — `defer resp.Body.Close()`
- `server.go:443` — `defer conn.Close()`
- `manager.go:139` — `defer f.Close()`
- `integration_test.go:25` — `ln.Close()` inside `t.Cleanup`
- `integration_test.go:33` — `defer conn.Close()` inside goroutine
- `integration_test.go:50` — `pc.Close()` inside `t.Cleanup`
- `integration_test.go:116` — `defer conn.Close()`
- `integration_test.go:158` — `defer clientConn.Close()`

**B — Operational `Close()` calls where logging the error adds value**:
- `server.go:268` — `ts.listener.Close()` in `RemoveTarget` → log error
- `server.go:276` — `uls.listenConn.Close()` in `RemoveTarget` → log error

## Acceptance Criteria

- [ ] `golangci-lint` passes with zero `errcheck` violations
- [ ] `go test -race -count=1 ./...` still passes

## Dependencies

- Requires REQ-031 (CI workflow) to verify fix

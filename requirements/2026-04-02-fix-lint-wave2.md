# Fix Second Wave of golangci-lint Violations

**Date Added**: 2026-04-02
**Priority**: High
**Status**: Completed

## Problem Statement

After REQ-032, a further 12 golangci-lint violations were found on the updated main branch: 9 `errcheck`, 1 `staticcheck S1000` (empty select), and 2 `staticcheck SA1019` (deprecated Docker API field).

## Functional Requirements

All lint violations resolved so CI lint job passes.

## Technical Requirements

- `server.go:509` `defer upstream.Close()` → `//nolint:errcheck`
- `server.go:521-522` `conn.Close()` / `upstream.Close()` in `closeOnce.Do` → `//nolint:errcheck`
- `server.go:435` empty `select { default: }` → remove (no-op, leftover code)
- `server_test.go:183,212,385,423` `defer *.Close()` → `//nolint:errcheck`
- `udp.go:189` `flow.upstreamConn.Close()` → log error (operational)
- `main.go:75` `fmt.Fprint(w, "ok")` → `//nolint:errcheck`
- `manager.go:408-409` deprecated `NetworkSettings.IPAddress` → remove last-resort block (already covered by Networks loop)

## Acceptance Criteria

- [ ] `golangci-lint run ./...` passes with zero violations
- [ ] `go test -race -count=1 ./...` still passes

## Dependencies

- Follows REQ-032 (first wave of errcheck fixes)

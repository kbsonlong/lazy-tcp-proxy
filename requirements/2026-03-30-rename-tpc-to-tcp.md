# Rename: tpc → tcp Throughout

**Date Added**: 2026-03-30
**Priority**: High
**Status**: Completed

## Problem Statement

The project subfolder, Go module path, binary name, Docker labels, and log strings all use `tpc` instead of `tcp`. This is a typo that causes real misconfiguration errors (users naturally write `lazy-tcp-proxy` labels and get no feedback).

## Functional Requirements

Rename every `tpc` occurrence to `tcp` across the repository:

- Subfolder: `lazy-tpc-proxy/` → `lazy-tcp-proxy/`
- Go module: `github.com/mountain-pass/lazy-tpc-proxy` → `github.com/mountain-pass/lazy-tcp-proxy`
- Import paths in all `.go` files
- Docker labels used in code: `lazy-tpc-proxy.enabled` → `lazy-tcp-proxy.enabled`, `lazy-tpc-proxy.port` → `lazy-tcp-proxy.port`
- Binary name in Dockerfile: `lazy-tpc-proxy` → `lazy-tcp-proxy`
- Log strings in `main.go`
- Requirement files that reference `lazy-tpc-proxy`

## Acceptance Criteria

- [ ] `grep -r "lazy-tpc" .` returns no matches.
- [ ] `go build ./...` passes from the renamed folder.
- [ ] Dockerfile builds successfully.

## Dependencies

- REQ-001, REQ-005 — prerequisite for correct label names.

## Implementation Notes

- User explicitly requested this rename; design/plan approval phases are noted as skipped per user instruction.

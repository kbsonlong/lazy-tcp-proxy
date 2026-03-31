# Yellow Container Names in Log Output — Implementation Plan

**Requirement**: [2026-03-31-yellow-container-names.md](2026-03-31-yellow-container-names.md)
**Date**: 2026-03-31
**Status**: Implemented

## Implementation Steps

1. In `lazy-tcp-proxy/internal/proxy/server.go` — wrap `%s` with `\033[33m` / `\033[0m` in every `log.Printf` format string whose corresponding argument is a container name (11 call sites).
2. In `lazy-tcp-proxy/internal/docker/manager.go` — same treatment for every `log.Printf` that prints a container name (5 call sites, including the joined-list line for `init: found containers`).
3. Run `go build ./...` to confirm no compilation errors.
4. Commit and push.

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `lazy-tcp-proxy/internal/proxy/server.go` | Modify | Embed yellow ANSI codes around `%s` at 11 log call sites |
| `lazy-tcp-proxy/internal/docker/manager.go` | Modify | Embed yellow ANSI codes around `%s` at 5 log call sites |

## API Contracts

N/A

## Data Models

N/A

## Key Code Snippets

```go
// Pattern applied at every affected log site
log.Printf("proxy: new connection to \033[33m%s\033[0m (port %d) from %s",
    ts.info.ContainerName, ts.targetPort, conn.RemoteAddr())
```

## Unit Tests

No automated tests for log output formatting; acceptance criteria verified by visual inspection of log lines and successful `go build`.

## Risks & Open Questions

- Terminals that do not support ANSI codes will display the raw escape sequences. This is acceptable for a developer-facing proxy tool running inside Docker.

## Deviations from Plan

None — implemented exactly as planned.

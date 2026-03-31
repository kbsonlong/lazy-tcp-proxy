# Cyan Source IP Address in Connection Logs — Implementation Plan

**Requirement**: [2026-03-31-cyan-source-ip.md](2026-03-31-cyan-source-ip.md)
**Date**: 2026-03-31
**Status**: Draft

## Implementation Steps

1. **`internal/proxy/server.go` line 215** — wrap `conn.RemoteAddr()` in cyan ANSI escape codes in the `log.Printf` call inside `handleConn`.

   Before:
   ```go
   log.Printf("proxy: new connection to \033[33m%s\033[0m (port %d) from %s",
       ts.info.ContainerName, ts.targetPort, conn.RemoteAddr())
   ```
   After:
   ```go
   log.Printf("proxy: new connection to \033[33m%s\033[0m (port %d) from \033[36m%s\033[0m",
       ts.info.ContainerName, ts.targetPort, conn.RemoteAddr())
   ```

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `lazy-tcp-proxy/internal/proxy/server.go` | Modify | Add `\033[36m`/`\033[0m` around `conn.RemoteAddr()` on the new-connection log line |

## Unit Tests

| Test | Input | Expected Output |
|------|-------|-----------------|
| Visual inspection | Connect to any proxied port | Log line shows source IP:port in cyan |

## Risks & Open Questions

None — single character-level change to a format string.

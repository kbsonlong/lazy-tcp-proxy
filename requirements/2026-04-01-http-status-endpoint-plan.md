# HTTP Status Endpoint — Implementation Plan

**Requirement**: [2026-04-01-http-status-endpoint.md](2026-04-01-http-status-endpoint.md)
**Date**: 2026-04-01
**Status**: Draft

## Implementation Steps

1. **Add `TargetSnapshot` struct to `internal/proxy/server.go`** — a plain, lock-free struct that mirrors `targetState` fields safe to expose externally. Include `ContainerID`, `ContainerName`, `ListenPort`, `TargetPort`, `Running`, `ActiveConns`, and `LastActive *time.Time` (pointer so zero marshals as `null`).

2. **Add `Snapshot() []TargetSnapshot` method to `ProxyServer`** — acquires the read lock, iterates `s.targets`, copies each field into a `TargetSnapshot`, and returns the slice. Container ID is trimmed to 12 chars here.

3. **Add `resolveStatusPort()` helper to `main.go`** — reads `STATUS_PORT` env var; defaults to `8080`; returns `-1` (disabled) when value is `"0"` or `0`; logs a warning and uses the default for invalid values.

4. **Add `runStatusServer()` function to `main.go`** — constructs an `http.ServeMux` with two routes:
   - `GET /status` — calls `srv.Snapshot()`, marshals to indented JSON, writes `Content-Type: application/json` + `200`.
   - `GET /health` — writes `200 OK` with body `ok`.
   Starts `http.Server.ListenAndServe` in a goroutine. Wires `http.Server.Shutdown` to a `context.AfterFunc` on the root context so it drains cleanly on shutdown.

5. **Wire into `main()`** — after creating `srv`, call `resolveStatusPort()`; if not disabled, call `runStatusServer(ctx, srv, port)` and log `status endpoint: :<port>`.

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `lazy-tcp-proxy/internal/proxy/server.go` | Modify | Add `TargetSnapshot` struct and `Snapshot()` method |
| `lazy-tcp-proxy/main.go` | Modify | Add `resolveStatusPort()`, `runStatusServer()`, wire into `main()` |

## API Contracts

### `GET /status`

**Response** `200 application/json`
```json
[
  {
    "container_id": "a1b2c3d4e5f6",
    "container_name": "my-service",
    "listen_port": 8080,
    "target_port": 80,
    "running": true,
    "active_conns": 2,
    "last_active": "2026-04-01T12:34:56Z"
  },
  {
    "container_id": "b2c3d4e5f6a1",
    "container_name": "idle-service",
    "listen_port": 9090,
    "target_port": 9000,
    "running": false,
    "active_conns": 0,
    "last_active": null
  }
]
```

### `GET /health`

**Response** `200 text/plain`
```
ok
```

## Data Models

```go
// TargetSnapshot is a point-in-time copy of a single port mapping's state,
// safe to read without holding any lock.
type TargetSnapshot struct {
    ContainerID   string     `json:"container_id"`
    ContainerName string     `json:"container_name"`
    ListenPort    int        `json:"listen_port"`
    TargetPort    int        `json:"target_port"`
    Running       bool       `json:"running"`
    ActiveConns   int32      `json:"active_conns"`
    LastActive    *time.Time `json:"last_active"`
}
```

## Key Code Snippets

```go
// Snapshot returns a point-in-time copy of all registered targets.
func (s *ProxyServer) Snapshot() []TargetSnapshot {
    s.mu.RLock()
    defer s.mu.RUnlock()
    out := make([]TargetSnapshot, 0, len(s.targets))
    for listenPort, ts := range s.targets {
        var la *time.Time
        if !ts.lastActive.IsZero() {
            t := ts.lastActive
            la = &t
        }
        out = append(out, TargetSnapshot{
            ContainerID:   ts.info.ContainerID[:12],
            ContainerName: ts.info.ContainerName,
            ListenPort:    listenPort,
            TargetPort:    ts.targetPort,
            Running:       ts.running,
            ActiveConns:   ts.activeConns.Load(),
            LastActive:    la,
        })
    }
    return out
}
```

```go
func runStatusServer(ctx context.Context, srv *proxy.ProxyServer, port int) {
    mux := http.NewServeMux()
    mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        enc := json.NewEncoder(w)
        enc.SetIndent("", "  ")
        enc.Encode(srv.Snapshot()) //nolint:errcheck
    })
    mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        fmt.Fprint(w, "ok")
    })
    hs := &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: mux}
    context.AfterFunc(ctx, func() {
        shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        hs.Shutdown(shutCtx) //nolint:errcheck
    })
    go func() {
        if err := hs.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("status server: %v", err)
        }
    }()
}
```

## Unit Tests

| Test | Input | Expected Output |
|------|-------|-----------------|
| `/status` with no targets | empty proxy | `200`, body `[]` |
| `/status` with one running target | one registered running container | `200`, JSON array with `running: true`, `last_active: null` if never active |
| `/status` with one stopped target | one registered stopped container | `200`, JSON array with `running: false` |
| `/health` | any | `200`, body `ok` |
| `STATUS_PORT=0` | env var set to `"0"` | no HTTP server started, no port bound |
| `STATUS_PORT` invalid | env var set to `"abc"` | warning logged, default `8080` used |

## Risks & Open Questions

- `context.AfterFunc` requires Go 1.21+. Verify the module's `go` directive in `go.mod` is ≥ 1.21 before using it; otherwise use a `goroutine + select on ctx.Done()` fallback.
- If `STATUS_PORT` clashes with a managed container's TCP listen port, `ListenAndServe` will fail — `log.Fatalf` is appropriate here since it's a misconfiguration.
- Iteration order of `s.targets` (a map) is non-deterministic; response order will vary between calls. This is acceptable for an operational endpoint.

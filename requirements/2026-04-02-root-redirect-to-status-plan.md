# Root Redirect to /status — Implementation Plan

**Requirement**: [2026-04-02-root-redirect-to-status.md](2026-04-02-root-redirect-to-status.md)
**Date**: 2026-04-02
**Status**: Implemented

## Implementation Steps

1. Open `lazy-tcp-proxy/main.go`. In `runStatusServer` (line 65), after the `/health` handler (line 73–76) and before the `http.Server` construction (line 77), add:
   ```go
   mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
       http.Redirect(w, r, "/status", http.StatusMovedPermanently)
   })
   ```
2. Update the requirement file status to "Completed" and plan status to "Implemented".
3. Commit and push all changes.

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `lazy-tcp-proxy/main.go` | Modify | Add `"/"` catch-all handler that redirects to `/status` |
| `requirements/2026-04-02-root-redirect-to-status.md` | Modify | Mark Completed |
| `requirements/2026-04-02-root-redirect-to-status-plan.md` | Modify | Mark Implemented |
| `requirements/_index.md` | Modify | Update status to Completed |

## API Contracts

| Method | Path | Response |
|--------|------|----------|
| `*` | `/` | `301 Moved Permanently`, `Location: /status` |
| `GET` | `/status` | `200 OK`, JSON array (unchanged) |
| `GET` | `/health` | `200 OK`, body `ok` (unchanged) |

## Key Code Snippets

```go
mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
    http.Redirect(w, r, "/status", http.StatusMovedPermanently)
})
```

Note: In Go's `ServeMux`, `"/"` is a wildcard — it catches any path not matched by a more specific pattern. `/status` and `/health` are more specific and continue to work as before.

## Unit Tests

| Test | Input | Expected Output |
|------|-------|-----------------|
| Root redirect | `GET /` | 301, `Location: /status` |
| Unknown path | `GET /foo` | 301, `Location: /status` |
| Status unaffected | `GET /status` | 200, JSON body |
| Health unaffected | `GET /health` | 200, `ok` |

## Risks & Open Questions

- The `"/"` catch-all also redirects unknown paths (e.g. `/foo`) — this is acceptable for an internal endpoint and consistent with Go's `ServeMux` semantics.

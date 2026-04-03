# Migrate docker/docker → moby/moby/client — Implementation Plan

**Requirement**: [2026-04-03-migrate-docker-client-module.md](2026-04-03-migrate-docker-client-module.md)
**Date**: 2026-04-03
**Status**: Implemented

## Background: API Architecture Change

With Docker Engine v29, the module layout changed significantly. Option/request types that were previously in sub-packages are now defined in the `client` package itself. The mapping is:

| Old module | Old type | New location |
|---|---|---|
| `github.com/docker/docker/client` | `client.Client`, `client.Opt`, etc. | `github.com/moby/moby/client` |
| `github.com/docker/docker/api/types/container` | `container.ListOptions` | `client.ContainerListOptions` |
| `github.com/docker/docker/api/types/container` | `container.StartOptions` | `client.ContainerStartOptions` |
| `github.com/docker/docker/api/types/container` | `container.StopOptions` | `client.ContainerStopOptions` |
| `github.com/docker/docker/api/types/filters` | `filters.NewArgs()` / `filters.Args` | `make(client.Filters)` / `client.Filters` |
| `github.com/docker/docker/api/types/events` | `events.ListOptions` | `client.EventsListOptions` |
| `github.com/docker/docker/api/types/network` | `network.InspectOptions` | `client.NetworkInspectOptions` |
| `github.com/docker/docker/api/types/events` | `events.ContainerEventType` | `github.com/moby/moby/api/types/events` (unchanged) |

Constructor: `client.NewClientWithOpts(opts...)` → `client.New(opts...)`

## Implementation Steps

1. **Add new modules to go.mod** via `go get`:
   ```
   go get github.com/moby/moby/client@v0.3.0
   go get github.com/moby/moby/api@v1.54.0
   ```

2. **Update imports in `lazy-tcp-proxy/internal/docker/manager.go`**:

   Remove these imports:
   ```go
   "github.com/docker/docker/api/types/container"
   "github.com/docker/docker/api/types/events"
   "github.com/docker/docker/api/types/filters"
   "github.com/docker/docker/api/types/network"
   "github.com/docker/docker/client"
   ```

   Add these imports:
   ```go
   "github.com/moby/moby/api/types/events"
   "github.com/moby/moby/client"
   ```

3. **Update constructor** (line 118):
   ```go
   // Before
   cli, err := client.NewClientWithOpts(opts...)
   // After
   cli, err := client.New(opts...)
   ```

4. **Update Filters usage** (lines 179–180 and 429–434):
   ```go
   // Before
   f := filters.NewArgs()
   f.Add("label", "lazy-tcp-proxy.enabled=true")
   // After
   f := make(client.Filters)
   f.Add("label", "lazy-tcp-proxy.enabled=true")
   ```
   The `Add` method signature is unchanged — only the construction changes.

5. **Update `container.ListOptions`** (line 182):
   ```go
   // Before
   containers, err := m.cli.ContainerList(ctx, container.ListOptions{
   // After
   containers, err := m.cli.ContainerList(ctx, client.ContainerListOptions{
   ```

6. **Update `events.ListOptions`** (line 436):
   ```go
   // Before
   msgCh, errCh := m.cli.Events(ctx, events.ListOptions{Filters: f})
   // After
   msgCh, errCh := m.cli.Events(ctx, client.EventsListOptions{Filters: f})
   ```

7. **Update `network.InspectOptions`** (line 295):
   ```go
   // Before
   netInfo, err := m.cli.NetworkInspect(ctx, netID, network.InspectOptions{})
   // After
   netInfo, err := m.cli.NetworkInspect(ctx, netID, client.NetworkInspectOptions{})
   ```

8. **Update `container.StartOptions`** (line 363):
   ```go
   // Before
   if err := m.cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
   // After
   if err := m.cli.ContainerStart(ctx, containerID, client.ContainerStartOptions{}); err != nil {
   ```

9. **Update `container.StopOptions`** (line 375):
   ```go
   // Before
   if err := m.cli.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout}); err != nil {
   // After
   if err := m.cli.ContainerStop(ctx, containerID, client.ContainerStopOptions{Timeout: &timeout}); err != nil {
   ```

10. **Remove `github.com/docker/docker`** from go.mod and run `go mod tidy` to clean up indirect dependencies.

11. **Build and test**:
    ```
    go build ./...
    go test ./...
    ```

12. **Update requirement status** to Completed in both the requirement file and `_index.md`.

13. **Commit and push** all changes.

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `lazy-tcp-proxy/internal/docker/manager.go` | Modify | Update imports and 7 type references |
| `lazy-tcp-proxy/go.mod` | Modify | Replace `github.com/docker/docker` with `github.com/moby/moby/client` + `api` |
| `lazy-tcp-proxy/go.sum` | Modify | Updated automatically by `go mod tidy` |

## Implementation Notes (Actual vs Planned)

The API surface of `github.com/moby/moby/client v0.3.0` changed more than anticipated:

1. **`ContainerList` returns a wrapper**: `ContainerListResult{Items []container.Summary}` — not a bare slice. Fixed by ranging over `.Items`.
2. **`ContainerInspect` now takes options**: Added `client.ContainerInspectOptions{}` argument. Returns `ContainerInspectResult{Container container.InspectResponse}` — unwrapped via local `inspect := result.Container`.
3. **`ContainerStart`/`ContainerStop` return result types**: Changed `if err :=` to `if _, err :=` to discard the unused result.
4. **`NetworkConnect` takes an options struct**: `NetworkConnectOptions{Container: containerID}` — no longer takes containerID and endpoint config as separate positional args.
5. **`NetworkDisconnect` takes an options struct**: `NetworkDisconnectOptions{Container: containerID, Force: false}` — no longer takes positional args.
6. **`Events` returns a single `EventsResult`**: Has `.Messages` and `.Err` channels — not two bare return values.
7. **`EndpointSettings.IPAddress` is `netip.Addr`**: Changed `!= ""` comparisons to `.IsValid()` and added `.String()` for return values.
8. **`client.Filters.Add` mutates in place**: The risk flagged in the plan was a non-issue — existing `f.Add(...)` call style worked without reassignment.

## Risks & Open Questions

- The `client.Filters.Add` method must return `client.Filters` (i.e., support in-place mutation) for the existing call style `f.Add(...)` to compile without reassignment. If it only supports chaining (returning a new map), the call sites need to become `f = f.Add(...)`. Verify during build.
- `go mod tidy` may remove indirect dependencies that were previously pulled in by `github.com/docker/docker` — verify the build still compiles fully after tidy.

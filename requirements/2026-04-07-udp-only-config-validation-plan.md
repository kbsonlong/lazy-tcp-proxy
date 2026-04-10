# REQ-043: UDP-Only Config Validation Fix — Implementation Plan

**Requirement**: [2026-04-07-udp-only-config-validation.md](2026-04-07-udp-only-config-validation.md)
**Date**: 2026-04-07
**Status**: Implemented

## Implementation Steps

1. **Fix `containerToTargetInfo` in Docker manager** (`lazy-tcp-proxy/internal/docker/manager.go`, ~line 246)
   - Replace the hard-require of `lazy-tcp-proxy.ports` with a check that accepts the container when either `lazy-tcp-proxy.ports` OR `lazy-tcp-proxy.udp-ports` is present.
   - When `lazy-tcp-proxy.ports` is absent but `lazy-tcp-proxy.udp-ports` is present: skip TCP port parsing, set `ports = nil`.
   - When neither is present: return an error mentioning both labels.

2. **Fix event-handler early-rejection guard in Docker manager** (`lazy-tcp-proxy/internal/docker/manager.go`, ~line 483)
   - The guard currently does: check `hasPorts`; if false, log and `continue`.
   - Change to: accept if `lazy-tcp-proxy.ports` OR `lazy-tcp-proxy.udp-ports` is present with valid content.
   - Update the rejection log message to mention both labels.

3. **Fix `deploymentToTargetInfo` in Kubernetes backend** (`lazy-tcp-proxy/internal/k8s/backend.go`, ~line 239)
   - Same logic as step 1 but using the annotation map (`ann`) and "annotation" in error messages.
   - When `lazy-tcp-proxy.ports` absent but `lazy-tcp-proxy.udp-ports` present: set `ports = nil`.
   - When neither present: return error mentioning both annotations.

4. **Update README.md** — label table (line 59)
   - Change `lazy-tcp-proxy.ports` Required column from `Yes` to `Yes*`
   - Add a footnote: `* At least one of lazy-tcp-proxy.ports or lazy-tcp-proxy.udp-ports must be set.`
   - Do the same for `lazy-tcp-proxy.udp-ports` (currently `No` → `Yes*`).
   - Update the Overview tagline from "TCP+UDP proxy" to reflect that either protocol alone is sufficient.

5. **Update the requirement and index status** to `Completed`.

6. **Commit and push** all changes.

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `lazy-tcp-proxy/internal/docker/manager.go` | Modify | Two locations: `containerToTargetInfo` + event handler guard |
| `lazy-tcp-proxy/internal/k8s/backend.go` | Modify | One location: `deploymentToTargetInfo` |
| `README.md` | Modify | Label table: ports/udp-ports Required column + footnote |
| `requirements/2026-04-07-udp-only-config-validation.md` | Modify | Status → Completed |
| `requirements/_index.md` | Modify | Status → Completed |

## Key Code Snippets

### Docker manager — `containerToTargetInfo` (replace lines ~246–252)

```go
var ports []PortMapping
portsStr, hasPorts := inspect.Config.Labels["lazy-tcp-proxy.ports"]
udpPortsStr, hasUDP := inspect.Config.Labels["lazy-tcp-proxy.udp-ports"]
if !hasPorts && (!hasUDP || udpPortsStr == "") {
    return TargetInfo{}, fmt.Errorf("missing label lazy-tcp-proxy.ports or lazy-tcp-proxy.udp-ports")
}
if hasPorts {
    ports = parsePortMappings("lazy-tcp-proxy.ports", portsStr)
    if len(ports) == 0 {
        return TargetInfo{}, fmt.Errorf("label lazy-tcp-proxy.ports contains no valid port mappings")
    }
}
```

### Docker manager — event handler guard (replace lines ~483–503)

```go
portsVal, hasPorts := attrs["lazy-tcp-proxy.ports"]
udpPortsVal, hasUDPPorts := attrs["lazy-tcp-proxy.udp-ports"]
if !hasPorts && (udpPortsVal == "" || !hasUDPPorts) {
    log.Printf("docker: event: container %s started but not proxied: missing label lazy-tcp-proxy.ports or lazy-tcp-proxy.udp-ports", name)
    continue
}
// validate whichever label(s) are present
if hasPorts {
    valid := false
    for _, token := range strings.Split(portsVal, ",") {
        parts := strings.SplitN(strings.TrimSpace(token), ":", 2)
        if len(parts) == 2 {
            _, e1 := strconv.Atoi(strings.TrimSpace(parts[0]))
            _, e2 := strconv.Atoi(strings.TrimSpace(parts[1]))
            if e1 == nil && e2 == nil {
                valid = true
                break
            }
        }
    }
    if !valid {
        log.Printf("docker: event: container %s started but not proxied: invalid ports value %q", name, portsVal)
        continue
    }
}
```

### Kubernetes backend — `deploymentToTargetInfo` (replace lines ~239–245)

```go
var ports []types.PortMapping
portsStr, hasPorts := ann["lazy-tcp-proxy.ports"]
udpPortsStr := ann["lazy-tcp-proxy.udp-ports"]
if !hasPorts && udpPortsStr == "" {
    return types.TargetInfo{}, fmt.Errorf("missing annotation lazy-tcp-proxy.ports or lazy-tcp-proxy.udp-ports")
}
if hasPorts && portsStr != "" {
    ports = types.ParsePortMappings("lazy-tcp-proxy.ports", portsStr)
    if len(ports) == 0 {
        return types.TargetInfo{}, fmt.Errorf("annotation lazy-tcp-proxy.ports contains no valid port mappings")
    }
}
```

## Unit Tests

| Test | Input | Expected Output |
|------|-------|-----------------|
| UDP-only container, Docker | `udp-ports=5353:53`, no `ports` | Registered, no error |
| TCP-only container, Docker | `ports=9000:80`, no `udp-ports` | Registered (unchanged) |
| Both labels, Docker | `ports=9000:80`, `udp-ports=5353:53` | Registered (unchanged) |
| Neither label, Docker | no `ports`, no `udp-ports` | Error: "missing label ...ports or ...udp-ports" |
| UDP-only deployment, k8s | annotation `udp-ports=5353:53`, no `ports` | Registered, no error |
| Neither annotation, k8s | no `ports`, no `udp-ports` | Error: "missing annotation ...ports or ...udp-ports" |

## Risks & Open Questions

- The `TargetInfo.Ports` field may be `nil`/empty for UDP-only targets. Callers that range over `Ports` to bind listeners must handle this gracefully — but this is already the case since UDP-only was always a valid concept in the data model, just not in validation.
- The Docker event handler validates TCP ports inline (not via `parsePortMappings`). If `lazy-tcp-proxy.ports` is absent we skip that validation block; the full validation still runs in `containerToTargetInfo` afterwards, so correctness is preserved.

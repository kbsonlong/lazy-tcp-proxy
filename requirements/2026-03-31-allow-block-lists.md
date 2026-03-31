# Per-Service Allow-List and Block-List via Labels

**Date Added**: 2026-03-31
**Priority**: Medium
**Status**: Planned

## Problem Statement

There is currently no way to restrict which source IP addresses can connect to a proxied service. Operators need per-service access control (e.g. allow only a private subnet to reach an SSH server, or block a known bad IP from a Minecraft server) without requiring host-level firewall configuration.

## Functional Requirements

1. **`lazy-tcp-proxy.allow-list`** label — comma-delimited list of IPs and/or CIDR ranges.
   - If present and non-empty, **only** connections whose source IP matches at least one entry are forwarded; all others are silently dropped (connection closed, brief log line).
2. **`lazy-tcp-proxy.block-list`** label — comma-delimited list of IPs and/or CIDR ranges.
   - If present and non-empty, connections whose source IP matches at least one entry are silently dropped; all others proceed normally.
3. Both labels are **optional**. If neither is set, behaviour is unchanged.
4. Both labels may be set simultaneously. Evaluation order: **allow-list checked first** (if set), then **block-list** (if set). A connection must pass the allow-list check before the block-list is evaluated.
5. Each entry may be:
   - A plain IPv4 address: `192.168.1.5`
   - A plain IPv6 address: `::1`
   - A CIDR range: `192.168.0.0/16`, `10.0.0.0/8`, `fd00::/8`
6. Whitespace around commas and individual entries is ignored.
7. Invalid entries (unparseable IPs/CIDRs) are skipped with a warning log at startup.

## User Experience Requirements

Label configuration example:
```yaml
labels:
  - "lazy-tcp-proxy.enabled=true"
  - "lazy-tcp-proxy.ports=9002:2222"
  - "lazy-tcp-proxy.allow-list=192.168.0.0/16,127.0.0.1"
  - "lazy-tcp-proxy.block-list=172.29.0.3,155.248.209.22"
```

When a connection is dropped by the allow-list:
```
proxy: connection from 155.248.209.22:61000 to minecraft rejected: not in allow-list
```

When a connection is dropped by the block-list:
```
proxy: connection from 155.248.209.22:61000 to minecraft rejected: address is blocked
```

## Technical Requirements

- Parsing happens in `docker/manager.go` inside `containerToTargetInfo`, adding `AllowList` and `BlockList` fields to `TargetInfo`.
- Each field stores a slice of `parsedEntry` values (each is either a `net.IP` for plain addresses or a `*net.IPNet` for CIDRs).
- Filtering happens at the top of `proxy/server.go:handleConn`, before `EnsureRunning` is called (so blocked connections do **not** wake the container).
- The source IP is extracted from `conn.RemoteAddr()` using `net.SplitHostPort`.
- No external dependencies are required (`net` stdlib is sufficient).

## Acceptance Criteria

- [ ] Container with `allow-list=192.168.0.0/16` only forwards connections from that subnet.
- [ ] Container with `block-list=1.2.3.4` drops connections from that IP, allows all others.
- [ ] Container with both labels: allow-list is evaluated first, then block-list on the remaining set.
- [ ] Plain IP and CIDR entries both work in each list.
- [ ] Invalid entries produce a warning log and are skipped; the container is still registered.
- [ ] A blocked connection does NOT start the container (checked via `EnsureRunning` not being called).
- [ ] The rejected connection log line names the container and the reason.
- [ ] No regression on containers without either label.

## Dependencies

- REQ-001 (Core TCP Proxy)
- REQ-007 (Multi-Port Mappings) — `TargetInfo` is the struct being extended

## Implementation Notes

- `TargetInfo` gains two new fields: `AllowList []net.IPNet` (plain IPs stored as /32 or /128 networks) and `BlockList []net.IPNet`.
- Helper `parseIPList(s string) ([]net.IPNet, error)` in `docker/manager.go`.
- Per-port filtering: because each `targetState` holds a `TargetInfo`, both lists are naturally per-service (one service may have multiple port mappings but they all share the same `TargetInfo`).

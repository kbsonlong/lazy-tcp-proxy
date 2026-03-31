# Per-Service Allow-List and Block-List via Labels — Implementation Plan

**Requirement**: [2026-03-31-allow-block-lists.md](2026-03-31-allow-block-lists.md)
**Date**: 2026-03-31
**Status**: Draft

## Implementation Steps

1. **`internal/docker/manager.go` — extend `TargetInfo`**
   Add two fields to the struct:
   ```go
   AllowList []net.IPNet // empty = no restriction
   BlockList []net.IPNet // empty = no restriction
   ```

2. **`internal/docker/manager.go` — add `parseIPList` helper**
   New package-level function that parses a comma-delimited string of IPs/CIDRs into `[]net.IPNet`. Plain IPs are converted to /32 (IPv4) or /128 (IPv6) nets. Invalid entries are skipped with a `log.Printf` warning.
   ```go
   func parseIPList(label, s string) []net.IPNet {
       var nets []net.IPNet
       for _, raw := range strings.Split(s, ",") {
           entry := strings.TrimSpace(raw)
           if entry == "" {
               continue
           }
           // Try CIDR first
           _, ipNet, err := net.ParseCIDR(entry)
           if err == nil {
               nets = append(nets, *ipNet)
               continue
           }
           // Try plain IP
           ip := net.ParseIP(entry)
           if ip == nil {
               log.Printf("docker: label %s: ignoring invalid entry %q", label, entry)
               continue
           }
           bits := 32
           if ip.To4() == nil {
               bits = 128
           }
           nets = append(nets, net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
       }
       return nets
   }
   ```

3. **`internal/docker/manager.go` — populate `AllowList`/`BlockList` in `containerToTargetInfo`**
   After the ports parsing block, add:
   ```go
   var allowList, blockList []net.IPNet
   if v, ok := inspect.Config.Labels["lazy-tcp-proxy.allow-list"]; ok && v != "" {
       allowList = parseIPList("lazy-tcp-proxy.allow-list", v)
   }
   if v, ok := inspect.Config.Labels["lazy-tcp-proxy.block-list"]; ok && v != "" {
       blockList = parseIPList("lazy-tcp-proxy.block-list", v)
   }
   ```
   And include them in the returned `TargetInfo`:
   ```go
   return TargetInfo{
       ...,
       AllowList: allowList,
       BlockList: blockList,
   }, nil
   ```

4. **`internal/proxy/server.go` — add `ipBlocked` helper**
   Package-level function that checks a source address string against a `TargetInfo`:
   ```go
   func ipBlocked(remoteAddr string, info docker.TargetInfo) bool {
       host, _, err := net.SplitHostPort(remoteAddr)
       if err != nil {
           return false // can't parse; let through
       }
       ip := net.ParseIP(host)
       if ip == nil {
           return false
       }
       // Allow-list: if set, IP must match at least one entry
       if len(info.AllowList) > 0 {
           allowed := false
           for _, n := range info.AllowList {
               if n.Contains(ip) {
                   allowed = true
                   break
               }
           }
           if !allowed {
               return true
           }
       }
       // Block-list: if set, IP must NOT match any entry
       for _, n := range info.BlockList {
           if n.Contains(ip) {
               return true
           }
       }
       return false
   }
   ```

5. **`internal/proxy/server.go` — filter in `handleConn`**
   Immediately after the `log.Printf("proxy: new connection ...")` call (and before `EnsureRunning`), add:
   ```go
   suffix := ""
   if ipBlocked(conn.RemoteAddr().String(), ts.info) {
       log.Printf("proxy: new connection to \033[33m%s\033[0m (port %d) from \033[36m%s\033[0m \033[31m(blocked)\033[0m",
           ts.info.ContainerName, ts.targetPort, conn.RemoteAddr())
       return
   }
   ```
   Note: the original (unblocked) log.Printf already covers the non-blocked path; the blocked path logs its own line and returns immediately. The `activeConns` counter is decremented by the deferred function, which is correct.

   Actually, it is cleaner to build the log line once with a conditional suffix, then return early if blocked. Revised approach:
   ```go
   blocked := ipBlocked(conn.RemoteAddr().String(), ts.info)
   if blocked {
       log.Printf("proxy: new connection to \033[33m%s\033[0m (port %d) from \033[36m%s\033[0m \033[31m(blocked)\033[0m",
           ts.info.ContainerName, ts.targetPort, conn.RemoteAddr())
       return
   }
   log.Printf("proxy: new connection to \033[33m%s\033[0m (port %d) from \033[36m%s\033[0m",
       ts.info.ContainerName, ts.targetPort, conn.RemoteAddr())
   ```

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `lazy-tcp-proxy/internal/docker/manager.go` | Modify | Add `AllowList`/`BlockList` to `TargetInfo`; add `parseIPList`; populate fields in `containerToTargetInfo` |
| `lazy-tcp-proxy/internal/proxy/server.go` | Modify | Add `ipBlocked` helper; check in `handleConn` before `EnsureRunning`; apply cyan to source IP (REQ-021 combined here) |

## API Contracts

No HTTP API changes. Label interface:

| Label | Format | Example |
|-------|--------|---------|
| `lazy-tcp-proxy.allow-list` | Comma-separated IPs/CIDRs | `192.168.0.0/16,127.0.0.1` |
| `lazy-tcp-proxy.block-list` | Comma-separated IPs/CIDRs | `172.29.0.3,155.248.209.22` |

## Unit Tests

| Test | Input | Expected |
|------|-------|----------|
| No lists | any IP | passes through |
| Allow-list only, IP in list | `192.168.1.5` vs `192.168.0.0/16` | passes |
| Allow-list only, IP not in list | `10.0.0.1` vs `192.168.0.0/16` | blocked |
| Block-list only, IP in list | `1.2.3.4` vs `1.2.3.4` | blocked |
| Block-list only, IP not in list | `10.0.0.1` vs `1.2.3.4` | passes |
| Both lists, IP passes allow, passes block | `192.168.1.5` / allow=`192.168.0.0/16`, block=`10.0.0.0/8` | passes |
| Both lists, IP passes allow, fails block | `192.168.1.5` / allow=`192.168.0.0/16`, block=`192.168.1.5` | blocked |
| Both lists, IP fails allow | `10.0.0.1` / allow=`192.168.0.0/16`, block=`10.0.0.0/8` | blocked (allow-list rejects first) |
| Invalid entry in list | `"192.168.0.0/16,not-an-ip"` | warning logged, valid entry still applied |
| IPv6 plain address | `::1` vs `::1` in block-list | blocked |
| IPv6 CIDR | `fd00::1` vs `fd00::/8` in allow-list | passes |

## Risks & Open Questions

- `net` package is already imported in both files; no new dependencies needed.
- The `activeConns` counter is incremented before the block check (it's deferred from the top of `handleConn`). This is fine: the counter is decremented by the deferred call when the function returns, so it returns to zero correctly.

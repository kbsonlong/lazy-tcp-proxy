# Cyan Source IP Address in Connection Logs

**Date Added**: 2026-03-31
**Priority**: Low
**Status**: Completed

## Problem Statement

When a new connection is logged, the source IP and port (e.g. `155.248.209.22:61000`) appears in plain white text, making it visually hard to distinguish from other parts of the log line. Colouring the source address in cyan would improve at-a-glance readability, consistent with the existing colour convention (container names = yellow, network names = green).

## Functional Requirements

- In the `proxy: new connection to ... from <addr>` log line, the `<addr>` component (IP:port) must be rendered in **cyan** ANSI colour.

## User Experience Requirements

- Log line before: `proxy: new connection to minecraft (port 25565) from 155.248.209.22:61000`
- Log line after:  `proxy: new connection to \033[36mmminecraft\033[0m (port 25565) from \033[36m155.248.209.22:61000\033[0m`
  (container name remains yellow `\033[33m`, source address becomes cyan `\033[36m`)

## Technical Requirements

- Use ANSI escape code `\033[36m` (cyan) / `\033[0m` (reset).
- Change is limited to the single `log.Printf` call in `handleConn` in `internal/proxy/server.go`.

## Acceptance Criteria

- [ ] `proxy: new connection to ... from <addr>` log line shows the source address in cyan.
- [ ] Container name in the same line remains yellow.
- [ ] No other log lines are affected.

## Dependencies

- REQ-014 (Yellow Container Names) — colour convention already established.

## Implementation Notes

Single-line change in `internal/proxy/server.go:215`.

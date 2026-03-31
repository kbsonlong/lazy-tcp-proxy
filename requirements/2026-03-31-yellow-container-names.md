# Yellow Container Names in Log Output

**Date Added**: 2026-03-31
**Priority**: Low
**Status**: Completed

## Problem Statement

Container names (`test-npm-socket`, `test-openssh-server`, `test-whoami`, etc.) appear inline within log messages using the same plain text as the surrounding context. When scanning logs it is difficult to quickly spot which container a message refers to.

## Functional Requirements

All log messages that reference a target container name must render the name in yellow using ANSI escape codes (`\033[33m`…`\033[0m`) so it stands out visually from the rest of the log line.

## User Experience Requirements

- Container names must be visually distinct from surrounding log text in any terminal that supports ANSI colour.
- No change to log message wording or structure — only the colour of the name portion changes.
- Applies to every log site in both `proxy/server.go` and `docker/manager.go` that mentions a container name.

## Technical Requirements

- Use ANSI escape codes directly in `log.Printf` format strings: `\033[33m%s\033[0m`.
- No external colour library; no new helper function or package.
- Must not affect container ID references (e.g. `6d490e39bf32`) — only human-readable names.

## Acceptance Criteria

- [x] `proxy: registered target` log line shows container name in yellow.
- [x] `proxy: updated target` log line shows container name in yellow.
- [x] `proxy: removing target` log line shows container name in yellow.
- [x] `proxy: new connection to` log line shows container name in yellow.
- [x] `proxy: last connection to … closed` log line shows container name in yellow.
- [x] `proxy: connection to … closed` log line shows container name in yellow.
- [x] `proxy: could not start container` log line shows container name in yellow.
- [x] `proxy: exhausted retries connecting to` log line shows container name in yellow.
- [x] `proxy: inactivity: error stopping` log line shows container name in yellow.
- [x] `docker: init: found containers` log line shows container name(s) in yellow.
- [x] `docker: discover: failed to join networks for` log line shows container name in yellow.
- [x] `docker: event: container added` log line shows container name in yellow.
- [x] `docker: event: container stopped` log line shows container name in yellow.
- [x] `docker: event: container removed` log line shows container name in yellow.
- [x] Build passes (`go build ./...`).

## Dependencies

- REQ-004 (Structured Init and Change Logging) — extends the existing log output.

## Implementation Notes

ANSI codes embedded directly in format strings. The `docker: init: found containers` message wraps the entire comma-joined list in a single yellow span (commas appear yellow too, which is acceptable).

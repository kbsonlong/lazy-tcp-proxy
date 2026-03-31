# Green Network Names in Log Output

**Date Added**: 2026-03-31
**Priority**: Low
**Status**: Completed

## Problem Statement

Network names appear in log messages using the same plain text as surrounding context, making them hard to distinguish when scanning logs. Container names are already highlighted in yellow (REQ-014); applying a consistent colour convention to network names improves readability further.

## Functional Requirements

All log messages in `docker/manager.go` that reference a network name or ID must render the value in green using ANSI escape codes (`\033[32m`…`\033[0m`).

## User Experience Requirements

- Network names/IDs must be visually distinct from surrounding log text in any ANSI-capable terminal.
- No change to log message wording or structure — only the colour of the network value changes.
- Green is used to differentiate networks from container names (yellow), establishing a consistent colour convention.

## Technical Requirements

- Use ANSI escape codes directly in `log.Printf` format strings: `\033[32m%s\033[0m`.
- No external colour library; no new helper function or package.

## Acceptance Criteria

- [x] `docker: joining network` log line shows network name in green.
- [x] `docker: init: joined networks` log line shows network name(s) in green.
- [x] `docker: failed to join network` log line shows network name in green.
- [x] `docker: could not inspect network` log line shows network ID in green.
- [x] `docker: event: joined network` log line shows network name in green.
- [x] Build passes (`go build ./...`).

## Dependencies

- REQ-014 (Yellow Container Names) — extends the same colour-coding convention to networks.

## Implementation Notes

Five log call sites in `docker/manager.go`. All changes are confined to format strings only.

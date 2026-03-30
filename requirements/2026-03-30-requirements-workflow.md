# Requirements-First Development Workflow

**Date Added**: 2026-03-30
**Priority**: High
**Status**: Completed

## Problem Statement

Without a structured process, requirements are undocumented, development skips design review, and it becomes hard to track what was built, why, and whether it meets the original intent.

## Functional Requirements

- All development follows a Design → Plan → Build workflow with human approval gates between phases.
- Every change (including small fixes) is recorded as a requirement file.
- Requirements live in `requirements/` as one file per requirement, with a shared `_index.md` summary table.
- A companion plan file (`-plan.md`) is created during Phase 2 for each requirement.

## User Experience Requirements

- Claude (and other agents) automatically follow this workflow without needing reminders.
- The `AGENTS.md` file in the repo root serves as the authoritative workflow reference for AI agents.
- The `CLAUDE.md` file in the repo root instructs Claude Code specifically to follow `AGENTS.md`.

## Technical Requirements

- `AGENTS.md` — already present; defines the full workflow, templates, and rules.
- `CLAUDE.md` — created at repo root; points Claude Code to `AGENTS.md` and summarises key rules.
- `requirements/_index.md` — lightweight summary table with `REQ-NNN` IDs, titles, status, dates, and links.
- Requirement filenames: `YYYY-MM-DD-short-slug.md` (date-prefixed to avoid concurrent conflicts).
- Index is append-only to minimise git merge conflicts.

## Acceptance Criteria

- [x] `CLAUDE.md` exists at the repo root and references `AGENTS.md`.
- [x] `requirements/` directory exists with `_index.md`.
- [x] All work completed prior to this requirement is back-documented with requirement files.
- [x] `_index.md` lists all requirements with correct IDs, statuses, and links.

## Dependencies

None — this is a meta/process requirement.

## Implementation Notes

- `AGENTS.md` was provided by the user and already contained the full workflow definition.
- `CLAUDE.md` was created as a thin wrapper pointing to `AGENTS.md`.
- REQ-001 and REQ-002 were back-documented retrospectively as part of this requirement.

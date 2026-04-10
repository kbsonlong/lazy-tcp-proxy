# Reorganise Example Directory

**Date Added**: 2026-04-04
**Priority**: Low
**Status**: Completed

## Problem Statement

The `example/` directory contains only a Docker Compose file at its root. With a Kubernetes example planned (REQ-038), the root needs to be reorganised into subdirectories so each backend has its own folder.

## Functional Requirements

1. Move `example/docker-compose.yml` → `example/docker/docker-compose.yml`.
2. Update any README references from `example/docker-compose.yml` to `example/docker/docker-compose.yml`.

## Acceptance Criteria

- [ ] `example/docker/docker-compose.yml` exists with identical content.
- [ ] `example/docker-compose.yml` no longer exists.
- [ ] README references updated to reflect the new path.

## Dependencies

Prepares the directory structure for REQ-038 (Kubernetes backend example).

## Implementation Notes

Simple file move + README update. No functional change.

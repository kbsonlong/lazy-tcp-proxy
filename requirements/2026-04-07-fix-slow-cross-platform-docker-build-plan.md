# Fix Slow Cross-Platform Docker Build — Implementation Plan

**Requirement**: [2026-04-07-fix-slow-cross-platform-docker-build.md](2026-04-07-fix-slow-cross-platform-docker-build.md)
**Date**: 2026-04-07
**Status**: Approved

## Implementation Steps

1. **Modify `lazy-tcp-proxy/Dockerfile` — pin builder to host platform**
   - Change `FROM golang:1.25.8-alpine AS builder`
     to `FROM --platform=$BUILDPLATFORM golang:1.25.8-alpine AS builder`
   - Add two `ARG` declarations immediately after the `FROM` line:
     ```
     ARG TARGETARCH
     ARG TARGETVARIANT
     ```
   - Change the `RUN go build` line from:
     ```
     RUN CGO_ENABLED=0 GOOS=linux go build -a -trimpath -o lazy-tcp-proxy .
     ```
     to:
     ```
     RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} go build -a -trimpath -o lazy-tcp-proxy .
     ```

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `lazy-tcp-proxy/Dockerfile` | Modify | Pin builder stage to `$BUILDPLATFORM`; add `ARG TARGETARCH`/`ARG TARGETVARIANT`; pass `GOARCH`/`GOARM` to `go build` |

## Key Code Snippets

**Before:**
```dockerfile
FROM golang:1.25.8-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go test ./...
RUN CGO_ENABLED=0 GOOS=linux go build -a -trimpath -o lazy-tcp-proxy .
```

**After:**
```dockerfile
FROM --platform=$BUILDPLATFORM golang:1.25.8-alpine AS builder
ARG TARGETARCH
ARG TARGETVARIANT
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go test ./...
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} go build -a -trimpath -o lazy-tcp-proxy .
```

**Notes:**
- `$BUILDPLATFORM` is a built-in BuildKit ARG — no declaration needed.
- `TARGETARCH` values: `amd64`, `arm64`, `arm` (for both arm/v6 and arm/v7).
- `TARGETVARIANT` values: `` (empty for amd64/arm64), `v7`, `v6`.
- `${TARGETVARIANT#v}` strips the leading `v` → `7`, `6`, or `` — all valid for `GOARM`.
- When `GOARCH=amd64` or `arm64`, `GOARM` is ignored by the Go toolchain even if set to empty string.
- The `go test ./...` step runs on the host arch — this is acceptable since unit/integration tests also
  run in CI on the host arch anyway.

## Risks & Open Questions

- None. This is a well-established Dockerfile pattern for Go multi-platform builds.

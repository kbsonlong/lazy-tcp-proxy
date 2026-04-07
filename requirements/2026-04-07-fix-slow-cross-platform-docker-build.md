# Fix Slow Cross-Platform Docker Build (QEMU → Native Cross-Compilation)

**Date Added**: 2026-04-07
**Priority**: Medium
**Status**: In Progress

## Problem Statement

Building the Docker image for `linux/arm/v7` (and `linux/arm/v6`) via `docker buildx build` takes ~2300
seconds. The root cause is that the Dockerfile's `builder` stage does not specify
`--platform=$BUILDPLATFORM`, so BuildKit runs the entire Go compilation step under QEMU emulation of the
target ARM architecture. QEMU-emulated arm/v7 is extremely slow for CPU-intensive work like Go
compilation.

## Functional Requirements

- The `builder` stage in the Dockerfile MUST run natively on the host architecture (amd64 or arm64).
- The Go `go build` command MUST cross-compile to the correct target OS/arch/variant using Go's built-in
  cross-compiler (`GOOS`, `GOARCH`, `GOARM` env vars).
- All four platforms (`linux/amd64`, `linux/arm64`, `linux/arm/v7`, `linux/arm/v6`) must continue to be
  produced correctly.
- The final image (`FROM scratch`) must still be targeted to the correct platform.

## User Experience Requirements

- `hooked publish` build time should reduce from ~2300 seconds to a few minutes.
- Published images must remain functionally identical.

## Technical Requirements

- Add `--platform=$BUILDPLATFORM` to the `FROM golang:... AS builder` line.
- Declare `ARG TARGETARCH` and `ARG TARGETVARIANT` in the builder stage.
- Replace the hardcoded `go build` command with one that sets `GOARCH=${TARGETARCH}` and
  `GOARM=${TARGETVARIANT#v}` (strips the leading `v` from e.g. `v7`).
- `GOOS=linux` and `CGO_ENABLED=0` remain unchanged.
- The `go test ./...` step in the builder stage also runs on the host platform (acceptable — tests are
  already run separately in CI).

## Acceptance Criteria

- [ ] `FROM` line for builder stage includes `--platform=$BUILDPLATFORM`.
- [ ] `ARG TARGETARCH` and `ARG TARGETVARIANT` are declared before the `RUN go build` step.
- [ ] `go build` command uses `GOARCH=${TARGETARCH}` and `GOARM=${TARGETVARIANT#v}`.
- [ ] `docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7,linux/arm/v6` completes
  successfully and produces valid images for all four platforms.
- [ ] Build time for `linux/arm/v7` drops significantly (target: under 120 seconds from a warm cache).

## Dependencies

- `lazy-tcp-proxy/Dockerfile`

## Implementation Notes

`GOARM` is only meaningful when `GOARCH=arm`. When building for `linux/arm64` or `linux/amd64`,
`TARGETVARIANT` is empty, so `GOARM=` is harmless. Alternatively the `GOARM` assignment can be
conditional, but the empty-value approach is accepted by the Go toolchain.

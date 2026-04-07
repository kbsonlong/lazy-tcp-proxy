# Sort /status Services by Name — Implementation Plan

**Requirement**: [2026-04-07-sort-status-services-by-name.md](2026-04-07-sort-status-services-by-name.md)
**Date**: 2026-04-07
**Status**: Implemented

## Implementation Steps

1. **`lazy-tcp-proxy/internal/proxy/server.go`** — Add `sort` import and sort the `out` slice in `Snapshot()` by `ContainerName` (then `ContainerID` as tie-breaker), immediately before the `return out` statement.
2. **`README.md`** — Update the `GET /status` description to note that the array is sorted alphabetically by container name. Update the example JSON to reflect a realistic sorted order.

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `lazy-tcp-proxy/internal/proxy/server.go` | Modify | Add `sort` import; sort `out` slice before return in `Snapshot()` |
| `README.md` | Modify | Note sorted output in `/status` section |

## Key Code Snippets

```go
// At top of file — add "sort" to import block
import (
    ...
    "sort"
    ...
)

// Inside Snapshot(), after the for loop, before return:
sort.Slice(out, func(i, j int) bool {
    if out[i].ContainerName != out[j].ContainerName {
        return out[i].ContainerName < out[j].ContainerName
    }
    return out[i].ContainerID < out[j].ContainerID
})
return out
```

## API Contracts

`GET /status` — response is unchanged in shape; entries are now guaranteed to be sorted alphabetically by `container_name`, with `container_id` as a tie-breaker.

## Unit Tests

| Test | Input | Expected Output |
|------|-------|-----------------|
| Existing snapshot tests | existing | must still pass |
| (No new test needed — sort is deterministic and verified by inspection) |

## Risks & Open Questions

None — purely additive, no behaviour change beyond ordering.

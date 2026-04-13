# 级联依赖锁：避免 dependants 被误判空闲并在 /status 展示 — Implementation Plan

**Requirement**: [2026-04-13-cascade-lock-skip-idle-timeout.md](2026-04-13-cascade-lock-skip-idle-timeout.md)
**Date**: 2026-04-13
**Status**: Draft

## Implementation Steps

1. 在 `ProxyServer` 内新增级联持有状态存储：
   - `cascadeParents map[string]map[string]struct{}`：depID → {upstreamID}
2. 在级联 start 路径中建立持有关系：
   - 在 `cascadeStart(upstream)` 对每个 `depID` 调用 `addCascadeLock(depID, upstreamID)`
   - 无论 dependant 是否已运行，都应建立持有关系，保证其不会被 idle-timeout 误停
3. 在级联 stop 路径中释放持有关系并决定是否停止：
   - 在 `cascadeStop(upstream)` 对每个 `depID` 调用 `removeCascadeLock(depID, upstreamID)`
   - 仅当 `depID` 的持有者集合为空（不再被任何上游持有）时，才执行 `StopContainer`
4. 在空闲检测中跳过被级联持有的容器：
   - 在 `checkInactivity` 中构建一次 `cascadeLockedIDs` 快照
   - TCP/UDP 聚合时若容器在 `cascadeLockedIDs` 中，则跳过 idle-timeout 判定
5. `/status` 增加字段：
   - `TargetSnapshot` 新增 `CascadeLocked bool \`json:"cascade_locked"\``
   - `Snapshot()` 根据 `cascadeParents` 填充该字段
6. 补充单元测试：
   - `checkInactivity` 对 cascade_locked 的容器不触发 StopContainer
   - `Snapshot()` 正确输出 `cascade_locked`
7. 文档更新：
   - 在 `docs/configuration.md` 的 status 相关描述中补充 `cascade_locked` 字段含义

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `lazy-tcp-proxy/internal/proxy/server.go` | Modify | 引入级联持有状态、跳过 idle 检测、/status 增加字段 |
| `lazy-tcp-proxy/internal/proxy/server_test.go` | Modify | 增加 cascade lock 的单测覆盖 |
| `docs/configuration.md` | Modify | 补充 /status 的 `cascade_locked` 字段说明 |

## Risks & Open Questions

- 多上游共享 dependant 的处理：采用持有者集合语义，避免误停。
- 该机制仅解决“dependants 不经 proxy 导致 lastActive 不更新而误停”的问题，不改变现有数据面转发路径。

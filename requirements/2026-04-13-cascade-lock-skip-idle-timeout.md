# 级联依赖锁：避免 dependants 被误判空闲并在 /status 展示

**Date Added**: 2026-04-13
**Priority**: High
**Status**: Planned

## Problem Statement

当 A 服务声明 `lazy-tcp-proxy.dependants=B` 且 B 并不通过 `lazy-tcp-proxy` 转发流量（例如 A 直连 B），当前实现会出现：
- A 通过 proxy 被访问时会级联启动 B（符合预期）
- 但 B 因没有经过 proxy 的连接，`lastActive` 长期为零值/不更新，会在空闲检测中被误判为“已空闲超时”并被停止（不符合预期）

需要引入一个“级联依赖锁”（cascade lock）的概念：当容器是作为 dependants 被上游持有时，自动跳过 idle-timeout 检测，避免误停止；并在 `/status` 输出中展示该容器当前是否处于级联持有状态。

## Functional Requirements

- 当容器 B 被某个上游容器 A 作为 dependants 级联启动（或被判定为需要随 A 一起运行）时：
  - 将 B 标记为 `cascade_locked=true`（可被多个上游同时持有）
  - 空闲检测不得因 idle-timeout 自动停止 B
- 当上游容器 A 因空闲或其他原因停止时：
  - 释放 A 对其 dependants 的级联持有
  - 仅当 B 不再被任何上游持有（锁计数为 0）时，才允许停止 B（停止动作由级联 stop 执行）
- `/status` 输出增加字段 `cascade_locked`，用于指示该条目对应的容器当前是否处于级联持有状态。

## User Experience Requirements

- 对于“直连依赖但需要启停联动”的服务栈（例如 immich 直连 redis/postgres），dependants 不应在上游仍在运行期间被误停止。
- 运维人员能通过 `/status` 直观看到某个容器是否是被级联持有状态。

## Technical Requirements

- 不引入新的第三方依赖。
- 级联持有应支持多上游共享同一 dependant（引用计数/集合语义）。
- 保持现有 `dependants` 的启动/停止语义兼容，仅修复“误停”与可观测性问题。

## Acceptance Criteria

- [ ] 当 A 级联启动 B 后，B 不会再被 idle-timeout 检测停止，直到 A 停止并触发级联 stop。
- [ ] 若 B 同时被多个上游持有，则只有当所有上游都停止后才会被停止。
- [ ] `GET /status` 返回的每个条目包含 `cascade_locked` 字段，且其值与运行时状态一致。

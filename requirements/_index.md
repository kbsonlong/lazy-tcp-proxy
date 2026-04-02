# Requirements Index

| ID      | Title                                      | Priority | Status    | Date Added | File                                                                           |
| ------- | ------------------------------------------ | -------- | --------- | ---------- | ------------------------------------------------------------------------------ |
| REQ-001 | Core TCP Proxy for Docker Containers       | High     | Completed | 2026-03-30 | [2026-03-30-core-tcp-proxy.md](2026-03-30-core-tcp-proxy.md)                   |
| REQ-002 | DOCKER_SOCK Env Var & Dockerfile Volume    | Medium   | Completed | 2026-03-30 | [2026-03-30-docker-sock-env-var.md](2026-03-30-docker-sock-env-var.md)         |
| REQ-003 | Requirements-First Development Workflow    | High     | Completed | 2026-03-30 | [2026-03-30-requirements-workflow.md](2026-03-30-requirements-workflow.md)     |
| REQ-004 | Structured Init and Change Logging         | Medium   | Completed | 2026-03-30 | [2026-03-30-structured-init-and-change-logging.md](2026-03-30-structured-init-and-change-logging.md) |
| REQ-005 | Log All Container Starts with Rejection Reason | High | Completed | 2026-03-30 | [2026-03-30-log-container-start-rejection.md](2026-03-30-log-container-start-rejection.md) |
| REQ-006 | Rename tpc → tcp Throughout                | High     | Completed | 2026-03-30 | [2026-03-30-rename-tpc-to-tcp.md](2026-03-30-rename-tpc-to-tcp.md) |
| REQ-007 | Multi-Port Mappings (ports label)          | High     | Completed | 2026-03-30 | [2026-03-30-multi-port-mappings.md](2026-03-30-multi-port-mappings.md) |
| REQ-008 | Keep Stopped Containers Registered         | High     | Completed | 2026-03-30 | [2026-03-30-keep-stopped-containers-registered.md](2026-03-30-keep-stopped-containers-registered.md) |
| REQ-009 | Fix Container Idle Timeout                 | High     | Completed | 2026-03-30 | [2026-03-30-fix-container-idle-timeout.md](2026-03-30-fix-container-idle-timeout.md) |
| REQ-010 | Idle-Timeout Observability & Poll Interval | Medium   | Completed | 2026-03-30 | [2026-03-30-idle-timeout-observability.md](2026-03-30-idle-timeout-observability.md) |
| REQ-011 | Fix Bidirectional TCP Proxy Teardown       | High     | Completed | 2026-03-30 | [2026-03-30-fix-proxy-teardown.md](2026-03-30-fix-proxy-teardown.md) |
| REQ-012 | Fix Redundant Container Stop Calls         | High     | Completed | 2026-03-30 | [2026-03-30-fix-redundant-stop.md](2026-03-30-fix-redundant-stop.md) |
| REQ-013 | Configurable Idle Timeout (IDLE_TIMEOUT_SECS) | Medium | Completed | 2026-03-30 | [2026-03-30-configurable-idle-timeout.md](2026-03-30-configurable-idle-timeout.md) |
| REQ-014 | Yellow Container Names in Log Output          | Low    | Completed | 2026-03-31 | [2026-03-31-yellow-container-names.md](2026-03-31-yellow-container-names.md) |
| REQ-015 | Container Name in Start/Stop Log Messages     | Low    | Completed | 2026-03-31 | [2026-03-31-container-name-in-start-stop-logs.md](2026-03-31-container-name-in-start-stop-logs.md) |
| REQ-016 | Green Network Names in Log Output             | Low    | Completed | 2026-03-31 | [2026-03-31-green-network-names.md](2026-03-31-green-network-names.md) |
| REQ-017 | Leave Joined Networks on Shutdown             | Medium | Completed | 2026-03-31 | [2026-03-31-leave-networks-on-shutdown.md](2026-03-31-leave-networks-on-shutdown.md) |
| REQ-018 | Reduce Proxy Memory via Buffer Pooling & Idle GC | Medium | Completed | 2026-03-31 | [2026-03-31-reduce-proxy-memory.md](2026-03-31-reduce-proxy-memory.md) |
| REQ-019 | Fix Dependabot Security Alerts (docker + otel)   | High   | Completed   | 2026-03-31 | [2026-03-31-fix-dependabot-security-alerts.md](2026-03-31-fix-dependabot-security-alerts.md) |
| REQ-020 | Fix CVE-2025-54410: Upgrade docker/docker to v28  | High   | Completed   | 2026-03-31 | [2026-03-31-fix-docker-cve-2025-54410.md](2026-03-31-fix-docker-cve-2025-54410.md) |
| REQ-021 | Cyan Source IP Address in Connection Logs         | Low    | Completed   | 2026-03-31 | [2026-03-31-cyan-source-ip.md](2026-03-31-cyan-source-ip.md) |
| REQ-022 | Per-Service Allow-List and Block-List via Labels  | Medium | Completed   | 2026-03-31 | [2026-03-31-allow-block-lists.md](2026-03-31-allow-block-lists.md) |
| REQ-023 | Discovered/Registered Containers Start as Idle    | High   | Completed   | 2026-04-01 | [2026-04-01-discovered-containers-start-idle.md](2026-04-01-discovered-containers-start-idle.md) |
| REQ-024 | Handle Port Conflicts Between Containers          | High   | Completed   | 2026-04-01 | [2026-04-01-handle-port-conflicts.md](2026-04-01-handle-port-conflicts.md) |
| REQ-025 | HTTP Status Endpoint (List Managed Containers)    | High   | Completed   | 2026-04-01 | [2026-04-01-http-status-endpoint.md](2026-04-01-http-status-endpoint.md) |
| REQ-026 | Webhook Support for Container Lifecycle Events    | Medium | Completed   | 2026-04-01 | [2026-04-01-webhook-support.md](2026-04-01-webhook-support.md) |
| REQ-027 | UDP Traffic Support                               | Medium | Completed   | 2026-04-01 | [2026-04-01-udp-traffic-support.md](2026-04-01-udp-traffic-support.md) |
| REQ-028 | Integration Tests (TCP and UDP Proxy)             | Medium | Completed   | 2026-04-01 | [2026-04-01-integration-tests.md](2026-04-01-integration-tests.md) |
| REQ-029 | Root Redirect to /status                          | Low    | Completed   | 2026-04-02 | [2026-04-02-root-redirect-to-status.md](2026-04-02-root-redirect-to-status.md) |
| REQ-030 | Last Active Default & Relative Time Field         | Medium | Completed   | 2026-04-02 | [2026-04-02-last-active-relative.md](2026-04-02-last-active-relative.md) |
| REQ-031 | GitHub Actions Go CI Workflow                     | High   | Completed   | 2026-04-02 | [2026-04-02-github-actions-go-ci.md](2026-04-02-github-actions-go-ci.md) |

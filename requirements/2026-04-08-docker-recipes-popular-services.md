# Docker Compose Recipes for Popular Service Images

**Date Added**: 2026-04-08
**Priority**: Medium
**Status**: Planned

## Problem Statement

The `recipes/` directory currently contains only 14 examples. Users looking to get started with popular services (databases, caches, message brokers, registries, CMS, etc.) have no ready-made recipe to copy from. Adding recipes for the most-pulled service images on Docker Hub reduces friction for new users.

## Functional Requirements

Create `docker-compose.*.yml` recipe files for the following services:

### On-demand TCP services (started lazily when a connection arrives)

1. **memcached** — port 11211 — distributed memory cache
2. **redis** — port 6379 — in-memory data store / cache
3. **mysql** — port 3306 — relational database
4. **rabbitmq** — ports 5672 (AMQP) + 15672 (management UI) — message broker
5. **mariadb** — port 3306 — MySQL-compatible relational database
6. **minio** — ports 9000 (S3 API) + 9001 (console) — S3-compatible object storage
7. **registry** — port 5000 — self-hosted Docker image registry
8. **wordpress** — port 8080 — WordPress CMS (includes MariaDB dependency)
9. **kafka** — port 9092 — distributed message streaming (KRaft mode, no ZooKeeper)
10. **verdaccio** — port 4873 — self-hosted npm/yarn registry
11. **pihole** — ports 53 (DNS UDP+TCP) + 8053 (admin web UI) — network-level ad blocker

### Cron-scheduled services (run on a schedule, web UI accessible via proxy)

12. **n8n** — port 5678 — workflow automation platform
    - `cron-start: "0 8 * * 1-5"` (Mon–Fri 08:00)
    - `cron-stop: "0 18 * * 1-5"` (Mon–Fri 18:00)
13. **uptime-kuma** — port 3001 — self-hosted uptime monitoring dashboard
    - `cron-start: "0 8 * * 1-5"` (Mon–Fri 08:00)
    - `cron-stop: "0 18 * * 1-5"` (Mon–Fri 18:00)

## User Experience Requirements

- Follow the naming convention: `docker-compose.<service>.<ports>.yml`
- Follow formatting patterns from existing recipes (healthchecks where applicable, env var defaults with `:-`, volumes for persistent data)
- Cron-scheduled recipes include a comment explaining why cron labels are used instead of (or in addition to) on-demand startup
- Wordpress recipe includes a bundled MariaDB service since WordPress cannot run standalone

## Technical Requirements

- Use pinned-but-stable image tags (e.g. `redis:alpine`, `mysql:8.4`, `mariadb:lts`)
- All services that store data must declare named volumes
- Services with a known health check command must include a `healthcheck:` block
- Kafka uses KRaft mode (no ZooKeeper dependency)
- Minio uses `minio/minio` (not an official library image) — note this in a comment
- n8n uses `n8nio/n8n` image
- Uptime Kuma uses `louislam/uptime-kuma` image

## Acceptance Criteria

- [ ] 13 new recipe files created under `recipes/`
- [ ] Each file follows the naming convention with correct port numbers in the filename
- [ ] Each file has `lazy-tcp-proxy.enabled=true` and `lazy-tcp-proxy.ports=...` labels (or cron labels for scheduled services)
- [ ] Cron-scheduled recipes (n8n, uptime-kuma) include both `lazy-tcp-proxy.cron-start` and `lazy-tcp-proxy.cron-stop` labels with a comment
- [ ] Wordpress recipe includes a MariaDB service and `depends_on` with health check condition
- [ ] All recipes with persistent data declare named volumes

## Dependencies

- Depends on: REQ-001 (core TCP proxy), REQ-007 (multi-port mappings), REQ-048 (cron scheduling) for n8n and uptime-kuma

## Implementation Notes

- Pihole: DNS runs on UDP 53 — the proxy handles this via the UDP support (REQ-035). Map port 53 for DNS and 8053 for the admin web UI.
- Kafka: Use `bitnami/kafka` in KRaft mode (single-node). This is not an official Docker Hub library image but is the de-facto standard.
- Wordpress multi-service compose: container name for the bundled MariaDB should be `wordpress-mariadb` to avoid conflict if the standalone `mariadb` recipe is also running.
- n8n and uptime-kuma: these are background services that need to run continuously to execute scheduled workflows / monitor targets. They do not start in response to TCP connections alone. Using `cron-start`/`cron-stop` (REQ-048) keeps them exempt from idle-timeout and ensures they run on a predictable schedule. The web UI is still accessible via the proxy while running.

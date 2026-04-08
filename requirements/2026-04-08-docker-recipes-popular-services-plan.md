# Docker Compose Recipes for Popular Service Images — Implementation Plan

**Requirement**: [2026-04-08-docker-recipes-popular-services.md](2026-04-08-docker-recipes-popular-services.md)
**Date**: 2026-04-08
**Status**: Approved

## Implementation Steps

1. Create `recipes/docker-compose.memcached.11211.yml`
2. Create `recipes/docker-compose.redis.6379.yml`
3. Create `recipes/docker-compose.mysql.3306.yml`
4. Create `recipes/docker-compose.rabbitmq.5672,15672.yml`
5. Create `recipes/docker-compose.mariadb.3306.yml`
6. Create `recipes/docker-compose.minio.9000,9001.yml`
7. Create `recipes/docker-compose.registry.5000.yml`
8. Create `recipes/docker-compose.wordpress.8080.yml`
9. Create `recipes/docker-compose.kafka.9092.yml`
10. Create `recipes/docker-compose.verdaccio.4873.yml`
11. Create `recipes/docker-compose.pihole.53,8053.yml`
12. Create `recipes/docker-compose.n8n.5678.yml`
13. Create `recipes/docker-compose.uptime-kuma.3001.yml`
14. Append REQ-053 row to `requirements/_index.md`
15. Commit and push all changes

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `recipes/docker-compose.memcached.11211.yml` | Create | memcached on port 11211 |
| `recipes/docker-compose.redis.6379.yml` | Create | Redis on port 6379, with volume |
| `recipes/docker-compose.mysql.3306.yml` | Create | MySQL 8.4 on port 3306, with volume + healthcheck |
| `recipes/docker-compose.rabbitmq.5672,15672.yml` | Create | RabbitMQ with management UI, ports 5672+15672, with volume |
| `recipes/docker-compose.mariadb.3306.yml` | Create | MariaDB LTS on port 3306, with volume + healthcheck |
| `recipes/docker-compose.minio.9000,9001.yml` | Create | MinIO S3 API + console, ports 9000+9001, with volume |
| `recipes/docker-compose.registry.5000.yml` | Create | Docker Registry v2 on port 5000, with volume |
| `recipes/docker-compose.wordpress.8080.yml` | Create | WordPress + bundled MariaDB, port 8080, with volumes |
| `recipes/docker-compose.kafka.9092.yml` | Create | Kafka KRaft (bitnami) on port 9092, with volume |
| `recipes/docker-compose.verdaccio.4873.yml` | Create | Verdaccio npm registry on port 4873, with volume |
| `recipes/docker-compose.pihole.53,8053.yml` | Create | Pi-hole DNS (53 UDP+TCP) + admin UI (8053), with volumes |
| `recipes/docker-compose.n8n.5678.yml` | Create | n8n with cron Mon–Fri 08:00–18:00, port 5678, with volume |
| `recipes/docker-compose.uptime-kuma.3001.yml` | Create | Uptime Kuma with cron Mon–Fri 08:00–18:00, port 3001, with volume |
| `requirements/_index.md` | Modify | Append REQ-053 row |

## Key Recipe Content

### memcached (11211)
```yaml
services:
  memcached:
    image: memcached:alpine
    container_name: memcached
    labels:
      - "lazy-tcp-proxy.enabled=true"
      - "lazy-tcp-proxy.ports=11211:11211"
```
No volume needed (cache is ephemeral by design).

### redis (6379)
```yaml
services:
  redis:
    image: redis:alpine
    container_name: redis
    volumes:
      - redis:/data
    labels:
      - "lazy-tcp-proxy.enabled=true"
      - "lazy-tcp-proxy.ports=6379:6379"
volumes:
  redis:
```

### mysql (3306)
```yaml
services:
  mysql:
    image: mysql:8.4
    container_name: mysql
    environment:
      MYSQL_ROOT_PASSWORD: ${MYSQL_ROOT_PASSWORD:-password}
      MYSQL_DATABASE: ${MYSQL_DATABASE:-mysql}
      MYSQL_USER: ${MYSQL_USER:-admin}
      MYSQL_PASSWORD: ${MYSQL_PASSWORD:-password}
    volumes:
      - mysql:/var/lib/mysql
    healthcheck:
      test: ["CMD", "mysqladmin", "ping", "-h", "localhost"]
      interval: 10s
      timeout: 5s
      retries: 5
    labels:
      - "lazy-tcp-proxy.enabled=true"
      - "lazy-tcp-proxy.ports=3306:3306"
volumes:
  mysql:
```

### rabbitmq (5672,15672)
Use `rabbitmq:management-alpine` to include the management UI in one image.
```yaml
services:
  rabbitmq:
    image: rabbitmq:management-alpine
    container_name: rabbitmq
    environment:
      RABBITMQ_DEFAULT_USER: ${RABBITMQ_DEFAULT_USER:-admin}
      RABBITMQ_DEFAULT_PASS: ${RABBITMQ_DEFAULT_PASS:-password}
    volumes:
      - rabbitmq:/var/lib/rabbitmq
    labels:
      - "lazy-tcp-proxy.enabled=true"
      - "lazy-tcp-proxy.ports=5672:5672,15672:15672"
volumes:
  rabbitmq:
```

### mariadb (3306)
```yaml
services:
  mariadb:
    image: mariadb:lts
    container_name: mariadb
    environment:
      MARIADB_ROOT_PASSWORD: ${MARIADB_ROOT_PASSWORD:-password}
      MARIADB_DATABASE: ${MARIADB_DATABASE:-mariadb}
      MARIADB_USER: ${MARIADB_USER:-admin}
      MARIADB_PASSWORD: ${MARIADB_PASSWORD:-password}
    volumes:
      - mariadb:/var/lib/mysql
    healthcheck:
      test: ["CMD", "healthcheck.sh", "--connect", "--innodb_initialized"]
      interval: 10s
      timeout: 5s
      retries: 5
    labels:
      - "lazy-tcp-proxy.enabled=true"
      - "lazy-tcp-proxy.ports=3306:3306"
volumes:
  mariadb:
```

### minio (9000,9001)
```yaml
services:
  minio:
    image: minio/minio
    container_name: minio
    environment:
      MINIO_ROOT_USER: ${MINIO_ROOT_USER:-admin}
      MINIO_ROOT_PASSWORD: ${MINIO_ROOT_PASSWORD:-password}
    command: server /data --console-address ":9001"
    volumes:
      - minio:/data
    labels:
      - "lazy-tcp-proxy.enabled=true"
      - "lazy-tcp-proxy.ports=9000:9000,9001:9001"
volumes:
  minio:
```

### registry (5000)
```yaml
services:
  registry:
    image: registry:2
    container_name: registry
    volumes:
      - registry:/var/lib/registry
    labels:
      - "lazy-tcp-proxy.enabled=true"
      - "lazy-tcp-proxy.ports=5000:5000"
volumes:
  registry:
```

### wordpress (8080) — multi-service
Bundles a `wordpress-mariadb` service (distinct name to avoid conflict with standalone mariadb recipe).
```yaml
services:
  wordpress:
    image: wordpress:latest
    container_name: wordpress
    environment:
      WORDPRESS_DB_HOST: wordpress-mariadb
      WORDPRESS_DB_USER: ${WORDPRESS_DB_USER:-wordpress}
      WORDPRESS_DB_PASSWORD: ${WORDPRESS_DB_PASSWORD:-password}
      WORDPRESS_DB_NAME: ${WORDPRESS_DB_NAME:-wordpress}
    volumes:
      - wordpress:/var/www/html
    depends_on:
      wordpress-mariadb:
        condition: service_healthy
    labels:
      - "lazy-tcp-proxy.enabled=true"
      - "lazy-tcp-proxy.ports=8080:80"

  wordpress-mariadb:
    image: mariadb:lts
    container_name: wordpress-mariadb
    environment:
      MARIADB_ROOT_PASSWORD: ${MARIADB_ROOT_PASSWORD:-password}
      MARIADB_DATABASE: ${WORDPRESS_DB_NAME:-wordpress}
      MARIADB_USER: ${WORDPRESS_DB_USER:-wordpress}
      MARIADB_PASSWORD: ${WORDPRESS_DB_PASSWORD:-password}
    volumes:
      - wordpress-db:/var/lib/mysql
    healthcheck:
      test: ["CMD", "healthcheck.sh", "--connect", "--innodb_initialized"]
      interval: 10s
      timeout: 5s
      retries: 5

volumes:
  wordpress:
  wordpress-db:
```

### kafka (9092) — KRaft mode
```yaml
services:
  kafka:
    image: bitnami/kafka
    container_name: kafka
    environment:
      - KAFKA_CFG_NODE_ID=0
      - KAFKA_CFG_PROCESS_ROLES=controller,broker
      - KAFKA_CFG_LISTENERS=PLAINTEXT://:9092,CONTROLLER://:9093
      - KAFKA_CFG_LISTENER_SECURITY_PROTOCOL_MAP=CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT
      - KAFKA_CFG_CONTROLLER_QUORUM_VOTERS=0@kafka:9093
      - KAFKA_CFG_CONTROLLER_LISTENER_NAMES=CONTROLLER
    volumes:
      - kafka:/bitnami/kafka
    labels:
      - "lazy-tcp-proxy.enabled=true"
      - "lazy-tcp-proxy.ports=9092:9092"
volumes:
  kafka:
```

### verdaccio (4873)
```yaml
services:
  verdaccio:
    image: verdaccio/verdaccio
    container_name: verdaccio
    volumes:
      - verdaccio:/verdaccio/storage
    labels:
      - "lazy-tcp-proxy.enabled=true"
      - "lazy-tcp-proxy.ports=4873:4873"
volumes:
  verdaccio:
```

### pihole (53,8053)
Pi-hole requires both TCP and UDP on port 53 for DNS. The admin web UI runs on port 80 inside the container, mapped to 8053 on the host.
```yaml
services:
  pihole:
    image: pihole/pihole
    container_name: pihole
    environment:
      TZ: ${TZ:-UTC}
      WEBPASSWORD: ${WEBPASSWORD:-password}
    volumes:
      - pihole-etc:/etc/pihole
      - pihole-dnsmasq:/etc/dnsmasq.d
    labels:
      - "lazy-tcp-proxy.enabled=true"
      - "lazy-tcp-proxy.ports=53:53,8053:80"
volumes:
  pihole-etc:
  pihole-dnsmasq:
```

### n8n (5678) — cron-scheduled
```yaml
services:
  n8n:
    image: n8nio/n8n
    container_name: n8n
    environment:
      - N8N_BASIC_AUTH_ACTIVE=true
      - N8N_BASIC_AUTH_USER=${N8N_BASIC_AUTH_USER:-admin}
      - N8N_BASIC_AUTH_PASSWORD=${N8N_BASIC_AUTH_PASSWORD:-password}
    volumes:
      - n8n:/home/node/.n8n
    labels:
      - "lazy-tcp-proxy.enabled=true"
      - "lazy-tcp-proxy.ports=5678:5678"
      # n8n runs workflows on a schedule - use cron labels so it runs
      # during business hours and is exempt from idle-timeout shutdown
      - "lazy-tcp-proxy.cron-start=0 8 * * 1-5"
      - "lazy-tcp-proxy.cron-stop=0 18 * * 1-5"
volumes:
  n8n:
```

### uptime-kuma (3001) — cron-scheduled
```yaml
services:
  uptime-kuma:
    image: louislam/uptime-kuma
    container_name: uptime-kuma
    volumes:
      - uptime-kuma:/app/data
    labels:
      - "lazy-tcp-proxy.enabled=true"
      - "lazy-tcp-proxy.ports=3001:3001"
      # uptime-kuma continuously polls monitored services - use cron labels
      # so it runs during business hours and is exempt from idle-timeout shutdown
      - "lazy-tcp-proxy.cron-start=0 8 * * 1-5"
      - "lazy-tcp-proxy.cron-stop=0 18 * * 1-5"
volumes:
  uptime-kuma:
```

## Risks & Open Questions

- Port 53 (pihole) requires elevated privileges on Linux hosts (`net.ipv4.ip_unprivileged_port_start` or running as root). Users may need to adjust host config. A comment in the recipe will note this.
- Port 9000/9001 (minio) conflicts with the existing `ollama` recipes if both are run simultaneously. Users should adjust ports as needed — this is noted by the port mapping in the filename.
- mariadb and mysql both use port 3306 — users cannot run both simultaneously without adjusting the host port.
- n8n basic auth env vars: `N8N_BASIC_AUTH_ACTIVE` is deprecated in newer n8n versions in favour of `N8N_SECURITY_AUDIT_DAYSABANDONEDWORKFLOW`. Using it anyway as it's the simplest credential setup; users can override.

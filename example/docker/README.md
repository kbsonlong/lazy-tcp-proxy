# lazy-tcp-proxy — Docker Example

This example runs lazy-tcp-proxy alongside five on-demand services using Docker Compose. All five services start at **zero running containers** and are started automatically the first time a connection arrives.

## Services

| Service | Protocol | Port | Notes |
|---------|----------|------|-------|
| whoami | TCP | 9001 | HTTP echo (headers, IP, etc.) |
| openssh-server | TCP | 9002 | SSH — user `admin`, password `password` |
| postgres | TCP | 5432 | User `admin`, password `password`, DB `mydatabase` |
| mongo | TCP | 27017 | User `admin`, password `password`, DB `mydatabase` |
| udp-echo | UDP | 9003 | Echoes any datagram back to the sender |

The idle timeout is **30 seconds** — a service stops automatically 30 seconds after the last connection closes.

## Prerequisites

- Docker and Docker Compose v2

## Start the example

```bash
cd example/docker
docker compose up -d
```

The proxy starts immediately. All target containers remain stopped until first use.

Check the proxy status (all services should show `running: false`):

```bash
curl -s http://localhost:8080/status | python3 -m json.tool
```

## Trigger on-demand scaling (0 → 1)

Each command below makes a connection through the proxy. Watch the container start in real time with `docker compose logs -f lazy-tcp-proxy`.

**whoami (HTTP):**
```bash
curl http://localhost:9001
```

**openssh-server (SSH):**
```bash
ssh admin@localhost -p 9002
# password: password
# type 'exit' to close the session
```

**postgres:**
```bash
psql -h localhost -p 5432 -U admin -d mydatabase
# password: password
# type '\q' to exit
```

**mongo:**
```bash
mongosh "mongodb://admin:password@localhost:27017/mydatabase?authSource=admin"
# type 'exit' to close
```

**udp-echo:**
```bash
echo "hello" | nc -u -w1 localhost 9003
# The first datagram starts the container; the server echoes it back.
```

## What to look for

**Proxy logs** — watch a container start on first connection:
```bash
docker compose logs -f lazy-tcp-proxy
```

You will see lines like:
```
proxy: new connection to whoami (port 80) from 172.x.x.x:xxxxx
docker: starting container whoami
docker: container whoami started
proxy: proxying connection to 172.x.x.x:80
proxy: last connection to whoami closed; idle timer started (container will stop in ~30s)
docker: stopping container whoami (idle timeout)
```

**Status endpoint** — poll to observe running state change:
```bash
watch -n2 'curl -s http://localhost:8080/status | python3 -m json.tool'
```

The `running` field flips from `false` → `true` on first connection and back to `false` after the idle timeout.

## Shutdown

```bash
docker compose down
```

Add `-v` to also remove the persistent volumes (postgres and mongo data):
```bash
docker compose down -v
```

---

## HTTP Ingress on 80 (Cloudflare Tunnel TLS) + Traefik + lazy-tcp-proxy

If most of your services are HTTP, you can avoid exposing a large port range by putting an L7 reverse proxy (Traefik) in front and only publishing port **80** on the host.

Traefik routes by Host header to `lazy-tcp-proxy` on an internal Docker network, and `lazy-tcp-proxy` then starts the real target container on-demand and forwards to its internal port.

### Start ingress stack

```bash
cd example/docker
docker compose -f docker-compose.http-ingress.yml up -d
```

This starts:
- `traefik` publishing `80:80`
- `lazy-tcp-proxy` (no published per-service HTTP ports; status is bound to `127.0.0.1:8080`)

### Configure routing (Traefik)

The example `traefik_dynamic.yml` is preconfigured for:
- `immich.kbsonlong.com`
- `ha.kbsonlong.com`
- `searx.kbsonlong.com`

If you want different names, edit `traefik_dynamic.yml` accordingly.

If you use Cloudflare Tunnel and terminate TLS at Cloudflare, configure the tunnel to forward these hostnames to:
- `http://<your-docker-host>:80`

### Configure targets (labels)

You must set labels at container creation time (Docker does not support updating labels on an existing container). If you already have these running, recreate them with labels in their own compose files.

Example labels (add these to each target container):

**immich-server → internal port 2283**
```yaml
labels:
  - "lazy-tcp-proxy.enabled=true"
  - "lazy-tcp-proxy.ports=9001:2283"
```

**homeassistant → internal port 8123**
```yaml
labels:
  - "lazy-tcp-proxy.enabled=true"
  - "lazy-tcp-proxy.ports=9002:8123"
```

**searxng → internal port 8080**
```yaml
labels:
  - "lazy-tcp-proxy.enabled=true"
  - "lazy-tcp-proxy.ports=9003:8080"
```

Once those containers exist with labels, `lazy-tcp-proxy` discovers them and begins listening on ports `9001-9003` inside the Docker network. Traefik can then reach them without publishing those ports on the host.

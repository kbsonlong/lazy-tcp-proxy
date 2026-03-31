
# lazy-tcp-proxy

**On-demand TCP proxy for Docker containers.**

> "Scale to zero!" - Nick G.

> "This is something that should really be built into Docker!" - Tom H.

`lazy-tcp-proxy` allows you to run many Dockerized services on a single host, but only start containers when a connection arrives. It stops containers after a configurable idle timeout, saving resources while providing seamless access.

---

## Features

- **Automatic TCP proxying:** Listens on host ports and proxies to containers, starting them on demand.
- **Label-based configuration:** Opt-in containers using Docker labels—no static config files.
- **Multi-port support:** Proxy multiple ports per container using `lazy-tcp-proxy.ports` label.
- **Idle shutdown:** Containers are stopped after a configurable period of inactivity.
- **Dynamic discovery:** Watches Docker events for new/removed containers and updates proxy targets live.
- **Network auto-join:** Proxy joins Docker networks as needed to reach containers by internal IP.
- **Graceful shutdown:** Leaves all joined networks on SIGINT/SIGTERM.
- **Structured, colorized logs:** Container names in yellow, network names in green for easy scanning.

---

## Quick Start

```sh
docker run -d \
	-v /var/run/docker.sock:/var/run/docker.sock \
    -e IDLE_TIMEOUT_SECS=30 \
    -e POLL_INTERVAL_SECS=5 \
	-p "9000-9999:9000-9999" \
    --restart=always \
    --name lazy-tcp-proxy \
	nickgrealy/lazy-tcp-proxy
```

Or use the provided `docker-compose.yml`.

---

## Building and Publishing

```sh
cd lazy-tcp-proxy
VERSION=1.`date +%Y%m%d`.`git rev-parse --short=8 HEAD`
docker buildx build \
  --platform linux/amd64,linux/arm64/v8 \
  --tag nickgrealy/lazy-tcp-proxy:${VERSION} \
  --tag nickgrealy/lazy-tcp-proxy:latest \
  --push \
  .
```

---

## Container Label Configuration

Add these labels to any container you want proxied:

- `lazy-tcp-proxy.enabled=true` (required)
- `lazy-tcp-proxy.ports=9000:80,9001:8080` (comma-separated `<listen>:<target>` pairs)

Example:

```yaml
labels:
	- "lazy-tcp-proxy.enabled=true"
	- "lazy-tcp-proxy.ports=9000:80,9001:8080"
```

---


## Environment Variables

| Variable            | Description                                                        | Default                   |
|---------------------|--------------------------------------------------------------------|---------------------------|
| `IDLE_TIMEOUT_SECS` | How long (in seconds) a container must be idle before being stopped| 120                       |
| `POLL_INTERVAL_SECS`| How often (in seconds) to check for idle containers                | 15                        |
| `DOCKER_SOCK`       | Path to Docker socket                                              | `/var/run/docker.sock`    |

All are optional; defaults are safe for most setups.

---

## Required resources

The container is designed to run with an extremely low footprint.

```shell
CONTAINER ID   NAME               CPU %     MEM USAGE / LIMIT     MEM %     NET I/O           BLOCK I/O         PIDS
cbc5f775a793   lazy-tcp-proxy     0.00%     4.238MiB / 19.52GiB   0.02%     1.51MB / 1.4MB    0B / 0B           13
```

---

## Logging

- **Container names** are shown in yellow: `\033[33m<name>\033[0m`
- **Network names** are shown in green: `\033[32m<name>\033[0m`
- All key events (startup, discovery, container start/stop, network join/leave, proxy activity) are logged with clear, structured messages.
- Rejection reasons for misconfigured containers are logged on every start event.

---

## Graceful Shutdown

On SIGINT or SIGTERM, the proxy disconnects itself from all joined Docker networks before exiting. All shutdown steps are logged.

---

## Requirements-First Development Workflow

All changes are tracked as requirements in the `requirements/` directory. See [AGENTS.md](AGENTS.md) for the full workflow. Every feature, fix, or change is documented and reviewed before implementation.

---

## Building & Development

- Written in Go, using the official Docker Go SDK.
- Minimal Docker image (`FROM scratch`).
- See requirements/ for detailed design and implementation notes.

---

## License

MIT
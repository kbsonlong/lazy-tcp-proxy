
# lazy-tcp-proxy

**On-demand TCP proxy for Docker containers.**

lazy-tcp-proxy allows you to run many Dockerized services on a single host, but only start containers when a connection arrives. It stops containers after a configurable idle timeout, saving resources while providing seamless access.

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
docker run \
	-v /var/run/docker.sock:/var/run/docker.sock \
	-e IDLE_TIMEOUT_SECS=120 \
	-e POLL_INTERVAL_SECS=15 \
	--network host \
	ghcr.io/nickgrealy/lazy-tcp-proxy:latest
```

Or use the provided `docker-compose.yml`.

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

- `IDLE_TIMEOUT_SECS` — How long (in seconds) a container must be idle before being stopped. Default: 120
- `POLL_INTERVAL_SECS` — How often (in seconds) to check for idle containers. Default: 15
- `DOCKER_SOCK` — Path to Docker socket. Default: `/var/run/docker.sock`

All are optional; defaults are safe for most setups.

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
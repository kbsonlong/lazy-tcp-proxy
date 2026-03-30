package docker

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

// PortMapping holds a single listen→target port pair.
type PortMapping struct {
	ListenPort int
	TargetPort int
}

// TargetInfo holds information about a proxy target container.
type TargetInfo struct {
	ContainerID   string
	ContainerName string
	Ports         []PortMapping
	NetworkIDs    []string
}

// TargetHandler is implemented by the proxy server to receive container updates.
type TargetHandler interface {
	RegisterTarget(info TargetInfo)
	RemoveTarget(containerID string)
	ContainerStopped(containerID string)
}

// Manager wraps the Docker client with proxy-specific logic.
type Manager struct {
	cli     *client.Client
	selfID  string
}

// NewManager creates a new DockerManager. The Docker socket path can be set via
// DOCKER_SOCK (e.g. /var/run/docker.sock). Falls back to DOCKER_HOST, then the
// default socket.
func NewManager() (*Manager, error) {
	opts := []client.Opt{client.FromEnv, client.WithAPIVersionNegotiation()}
	if sock := os.Getenv("DOCKER_SOCK"); sock != "" {
		opts = append([]client.Opt{client.WithHost("unix://" + sock)}, opts...)
	}
	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("creating docker client: %w", err)
	}

	m := &Manager{cli: cli}
	m.selfID = m.SelfContainerID()
	if m.selfID != "" {
		log.Printf("docker: detected self container ID: %s", m.selfID)
	} else {
		log.Printf("docker: could not detect self container ID; network auto-join disabled")
	}
	return m, nil
}

// SelfContainerID reads the proxy's own container ID from /proc/self/cgroup,
// falling back to /etc/hostname, and returns "" if not determinable.
func (m *Manager) SelfContainerID() string {
	// Try /proc/self/cgroup first (docker sets long hex IDs here)
	f, err := os.Open("/proc/self/cgroup")
	if err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			parts := strings.Split(line, ":")
			if len(parts) < 3 {
				continue
			}
			cgroupPath := parts[2]
			base := filepath.Base(cgroupPath)
			// Docker container IDs are 64-char hex strings
			if len(base) == 64 {
				return base
			}
			// Also handle docker-<id>.scope format used by systemd cgroups v2
			if strings.HasPrefix(base, "docker-") && strings.HasSuffix(base, ".scope") {
				id := strings.TrimPrefix(base, "docker-")
				id = strings.TrimSuffix(id, ".scope")
				if len(id) == 64 {
					return id
				}
			}
		}
	}

	// Fallback: /etc/hostname often contains the short container ID
	data, err := os.ReadFile("/etc/hostname")
	if err == nil {
		id := strings.TrimSpace(string(data))
		if len(id) == 12 || len(id) == 64 {
			return id
		}
	}

	return ""
}

// Discover lists all containers (running or stopped) that have the proxy label,
// joins their networks, and calls handler.RegisterTarget for each.
func (m *Manager) Discover(ctx context.Context, handler TargetHandler) error {
	f := filters.NewArgs()
	f.Add("label", "lazy-tcp-proxy.enabled=true")

	containers, err := m.cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: f,
	})
	if err != nil {
		return fmt.Errorf("listing containers: %w", err)
	}

	var foundNames []string
	var allNetworks []string
	for _, c := range containers {
		info, err := m.containerToTargetInfo(ctx, c.ID)
		if err != nil {
			log.Printf("docker: discover: skipping container %s: %v", c.ID[:12], err)
			continue
		}

		joined, err := m.JoinNetworks(ctx, info.NetworkIDs)
		if err != nil {
			log.Printf("docker: discover: failed to join networks for %s: %v", info.ContainerName, err)
		}
		allNetworks = append(allNetworks, joined...)

		handler.RegisterTarget(info)
		foundNames = append(foundNames, info.ContainerName)
	}

	if len(foundNames) == 0 {
		log.Printf("docker: init: no proxy containers found")
	} else {
		log.Printf("docker: init: found containers: %s", strings.Join(foundNames, ", "))
	}
	if len(allNetworks) == 0 {
		log.Printf("docker: init: no networks joined")
	} else {
		log.Printf("docker: init: joined networks: %s", strings.Join(allNetworks, ", "))
	}

	return nil
}

// containerToTargetInfo inspects a container and builds a TargetInfo.
func (m *Manager) containerToTargetInfo(ctx context.Context, containerID string) (TargetInfo, error) {
	inspect, err := m.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return TargetInfo{}, fmt.Errorf("inspecting container: %w", err)
	}

	portsStr, ok := inspect.Config.Labels["lazy-tcp-proxy.ports"]
	if !ok {
		return TargetInfo{}, fmt.Errorf("missing label lazy-tcp-proxy.ports")
	}
	var ports []PortMapping
	for _, token := range strings.Split(portsStr, ",") {
		parts := strings.SplitN(strings.TrimSpace(token), ":", 2)
		if len(parts) != 2 {
			return TargetInfo{}, fmt.Errorf("invalid port mapping %q: expected <listen>:<target>", token)
		}
		lp, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return TargetInfo{}, fmt.Errorf("invalid listen port in %q: %w", token, err)
		}
		tp, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return TargetInfo{}, fmt.Errorf("invalid target port in %q: %w", token, err)
		}
		ports = append(ports, PortMapping{ListenPort: lp, TargetPort: tp})
	}

	name := strings.TrimPrefix(inspect.Name, "/")

	var networkIDs []string
	for _, ep := range inspect.NetworkSettings.Networks {
		if ep.NetworkID != "" {
			networkIDs = append(networkIDs, ep.NetworkID)
		}
	}

	return TargetInfo{
		ContainerID:   containerID,
		ContainerName: name,
		Ports:         ports,
		NetworkIDs:    networkIDs,
	}, nil
}

// JoinNetworks connects the proxy container to each of the provided network IDs
// if it is not already a member. Returns the names of networks newly joined.
func (m *Manager) JoinNetworks(ctx context.Context, networkIDs []string) ([]string, error) {
	if m.selfID == "" {
		return nil, nil
	}

	var joined []string
	for _, netID := range networkIDs {
		// Inspect the network to check current membership
		netInfo, err := m.cli.NetworkInspect(ctx, netID, types.NetworkInspectOptions{})
		if err != nil {
			log.Printf("docker: could not inspect network %s: %v", netID, err)
			continue
		}

		// Check if we're already connected
		alreadyConnected := false
		for cid := range netInfo.Containers {
			if strings.HasPrefix(cid, m.selfID) || strings.HasPrefix(m.selfID, cid) {
				alreadyConnected = true
				break
			}
		}

		if alreadyConnected {
			continue
		}

		log.Printf("docker: joining network %s (%s)", netInfo.Name, netID[:12])
		if err := m.cli.NetworkConnect(ctx, netID, m.selfID, nil); err != nil {
			// Ignore "already exists" errors
			if !strings.Contains(err.Error(), "already exists") {
				log.Printf("docker: failed to join network %s: %v", netInfo.Name, err)
			}
		} else {
			joined = append(joined, netInfo.Name)
		}
	}

	return joined, nil
}

// EnsureRunning starts the container if it is not already running.
func (m *Manager) EnsureRunning(ctx context.Context, containerID string) error {
	inspect, err := m.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return fmt.Errorf("inspecting container: %w", err)
	}

	if inspect.State.Running {
		return nil
	}

	log.Printf("docker: starting container %s", containerID[:12])
	if err := m.cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("starting container: %w", err)
	}

	log.Printf("docker: container %s started", containerID[:12])
	return nil
}

// StopContainer stops the given container with a 10-second timeout.
func (m *Manager) StopContainer(ctx context.Context, containerID string) error {
	timeout := 10
	log.Printf("docker: stopping container %s (idle timeout)", containerID[:12])
	if err := m.cli.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout}); err != nil {
		return fmt.Errorf("stopping container: %w", err)
	}
	log.Printf("docker: container %s stopped", containerID[:12])
	return nil
}

// GetContainerIP returns the IP address of the container, preferring the given
// networkID. Falls back to any available network IP.
func (m *Manager) GetContainerIP(ctx context.Context, containerID, preferNetworkID string) (string, error) {
	inspect, err := m.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return "", fmt.Errorf("inspecting container: %w", err)
	}

	// Try the preferred network first
	if preferNetworkID != "" {
		for netID, ep := range inspect.NetworkSettings.Networks {
			_ = netID
			if ep.NetworkID == preferNetworkID && ep.IPAddress != "" {
				return ep.IPAddress, nil
			}
		}
	}

	// Fallback: any network with an IP
	for _, ep := range inspect.NetworkSettings.Networks {
		if ep.IPAddress != "" {
			return ep.IPAddress, nil
		}
	}

	// Last resort: top-level IP
	if inspect.NetworkSettings.IPAddress != "" {
		return inspect.NetworkSettings.IPAddress, nil
	}

	return "", fmt.Errorf("no IP address found for container %s", containerID[:12])
}

// WatchEvents subscribes to Docker events for containers with the proxy label.
// On create/start events it calls Discover; on die events it calls handler.RemoveTarget.
// It reconnects with exponential backoff on error.
func (m *Manager) WatchEvents(ctx context.Context, handler TargetHandler) {
	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		f := filters.NewArgs()
		f.Add("type", string(events.ContainerEventType))
		f.Add("event", "start")
		f.Add("event", "die")
		f.Add("event", "destroy")
		f.Add("event", "create")

		msgCh, errCh := m.cli.Events(ctx, types.EventsOptions{Filters: f})

		log.Printf("docker: watching events...")
		eventLoop := true
		for eventLoop {
			select {
			case <-ctx.Done():
				return
			case err := <-errCh:
				if err != nil {
					log.Printf("docker: events error: %v; reconnecting in %s", err, backoff)
					select {
					case <-ctx.Done():
						return
					case <-time.After(backoff):
					}
					backoff *= 2
					if backoff > maxBackoff {
						backoff = maxBackoff
					}
					eventLoop = false
				}
			case msg := <-msgCh:
				backoff = time.Second // reset on success
				switch msg.Action {
				case "create", "start":
					name := msg.Actor.Attributes["name"]
					attrs := msg.Actor.Attributes
					if attrs["lazy-tcp-proxy.enabled"] != "true" {
						log.Printf("docker: event: container %s started but not proxied: missing label lazy-tcp-proxy.enabled=true", name)
						continue
					}
					portsVal, hasPorts := attrs["lazy-tcp-proxy.ports"]
					if !hasPorts {
						log.Printf("docker: event: container %s started but not proxied: missing label lazy-tcp-proxy.ports", name)
						continue
					}
					valid := false
					for _, token := range strings.Split(portsVal, ",") {
						parts := strings.SplitN(strings.TrimSpace(token), ":", 2)
						if len(parts) == 2 {
							_, e1 := strconv.Atoi(strings.TrimSpace(parts[0]))
							_, e2 := strconv.Atoi(strings.TrimSpace(parts[1]))
							if e1 == nil && e2 == nil {
								valid = true
								break
							}
						}
					}
					if !valid {
						log.Printf("docker: event: container %s started but not proxied: invalid ports value %q", name, portsVal)
						continue
					}
					log.Printf("docker: event: container added: %s", name)
					info, err := m.containerToTargetInfo(ctx, msg.Actor.ID)
					if err != nil {
						log.Printf("docker: event: could not get target info for %s: %v", name, err)
						continue
					}
					joined, err := m.JoinNetworks(ctx, info.NetworkIDs)
					if err != nil {
						log.Printf("docker: event: failed to join networks: %v", err)
					}
					for _, n := range joined {
						log.Printf("docker: event: joined network: %s", n)
					}
					handler.RegisterTarget(info)

				case "die":
					name := msg.Actor.Attributes["name"]
					log.Printf("docker: event: container stopped: %s (still registered)", name)
					handler.ContainerStopped(msg.Actor.ID)

				case "destroy":
					name := msg.Actor.Attributes["name"]
					log.Printf("docker: event: container removed: %s", name)
					handler.RemoveTarget(msg.Actor.ID)
				}
			}
		}
	}
}

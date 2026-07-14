package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"backend/internal/model"
	"backend/internal/repository"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
)

// recreateContainer stops, removes and recreates a container from its
// captured config, preserving the full Config/HostConfig, named-volume
// mounts and network endpoints (aliases, static IPs). Secondary networks
// are connected before start so the container never runs disconnected.
func recreateContainer(
	ctx context.Context,
	repo repository.ContainerRepository,
	cfg model.ContainerConfig,
	progress func(line string),
) (string, error) {
	if cfg.Config == nil {
		return "", fmt.Errorf("captured config is empty")
	}

	timeout := 10
	progress("Stopping...")
	if err := repo.StopContainer(ctx, cfg.ID, &timeout); err != nil {
		return "", fmt.Errorf("stop: %w", err)
	}

	progress("Removing...")
	if err := repo.RemoveContainer(ctx, cfg.ID); err != nil {
		return "", fmt.Errorf("remove: %w", err)
	}

	hostCfg := cfg.HostConfig
	if hostCfg == nil {
		hostCfg = &container.HostConfig{}
	}
	ensureVolumeMounts(hostCfg, cfg)

	primary, secondary := splitEndpoints(cfg)
	var netCfg *network.NetworkingConfig
	if primary != "" {
		netCfg = &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				primary: sanitizeEndpoint(cfg.Endpoints[primary], cfg),
			},
		}
	}

	progress("Creating...")
	newID, err := repo.CreateContainer(ctx, cfg.Name, cfg.Config, hostCfg, netCfg)
	if err != nil {
		return "", fmt.Errorf("create: %w", err)
	}

	for _, name := range secondary {
		progress("Connecting network " + name + "...")
		ep := sanitizeEndpoint(cfg.Endpoints[name], cfg)
		if err := repo.ConnectNetwork(ctx, name, newID, ep); err != nil {
			progress(fmt.Sprintf("WARNING: connect network %s: %s", name, err))
		}
	}

	progress("Starting...")
	if err := repo.StartContainer(ctx, newID); err != nil {
		return newID, fmt.Errorf("start: %w", err)
	}
	return newID, nil
}

// ensureVolumeMounts re-adds volume and bind mounts that are visible in
// the inspect MountPoints but absent from HostConfig (docker moves
// compose-created volumes out of Binds on some engine versions).
func ensureVolumeMounts(hostCfg *container.HostConfig, cfg model.ContainerConfig) {
	covered := map[string]bool{}
	for _, b := range hostCfg.Binds {
		parts := strings.Split(b, ":")
		if len(parts) >= 2 {
			covered[parts[1]] = true
		}
	}
	for _, m := range hostCfg.Mounts {
		covered[m.Target] = true
	}

	for _, m := range cfg.Mounts {
		if covered[m.Destination] {
			continue
		}
		switch m.Type {
		case "volume":
			if m.Name == "" {
				continue
			}
			hostCfg.Mounts = append(hostCfg.Mounts, mount.Mount{
				Type:     mount.TypeVolume,
				Source:   m.Name,
				Target:   m.Destination,
				ReadOnly: !m.RW,
			})
		case "bind":
			hostCfg.Mounts = append(hostCfg.Mounts, mount.Mount{
				Type:     mount.TypeBind,
				Source:   m.Source,
				Target:   m.Destination,
				ReadOnly: !m.RW,
			})
		}
	}
}

// splitEndpoints picks the network passed at create time (the one named
// by NetworkMode, when it is a custom network) and the ones to connect
// afterwards.
func splitEndpoints(cfg model.ContainerConfig) (primary string, secondary []string) {
	var custom []string
	for name := range cfg.Endpoints {
		if !isDefaultNetwork(name) {
			custom = append(custom, name)
		}
	}
	sort.Strings(custom)

	mode := string(cfg.NetworkMode())
	if mode != "" && mode != "default" && !isDefaultNetwork(mode) {
		primary = mode
	}

	for _, name := range custom {
		if name != primary {
			secondary = append(secondary, name)
		}
	}
	return primary, secondary
}

// sanitizeEndpoint keeps only the user-intent fields of a captured
// endpoint; runtime fields (EndpointID, assigned IPs) must not be sent
// back to the engine.
func sanitizeEndpoint(ep *network.EndpointSettings, cfg model.ContainerConfig) *network.EndpointSettings {
	if ep == nil {
		return &network.EndpointSettings{}
	}
	shortID := cfg.ID
	if len(shortID) > 12 {
		shortID = shortID[:12]
	}
	clean := &network.EndpointSettings{
		IPAMConfig: ep.IPAMConfig,
		Links:      ep.Links,
		DriverOpts: ep.DriverOpts,
	}
	for _, alias := range ep.Aliases {
		if alias == shortID {
			continue
		}
		clean.Aliases = append(clean.Aliases, alias)
	}
	return clean
}

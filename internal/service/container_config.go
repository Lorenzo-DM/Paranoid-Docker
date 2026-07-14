package service

import (
	"context"
	"strings"

	"backend/internal/model"
	"backend/internal/repository"

	"github.com/docker/docker/api/types"
)

func captureContainerConfig(inspect types.ContainerJSON) model.ContainerConfig {
	cfg := model.ContainerConfig{
		Mounts: inspect.Mounts,
	}
	if inspect.ContainerJSONBase != nil {
		cfg.ID = inspect.ID
		cfg.Name = strings.TrimPrefix(inspect.Name, "/")
		cfg.HostConfig = inspect.HostConfig
	}
	if inspect.Config != nil {
		cfg.Config = inspect.Config
		cfg.Image = inspect.Config.Image
	}
	if inspect.NetworkSettings != nil {
		cfg.Endpoints = inspect.NetworkSettings.Networks
	}
	return cfg
}

// isDefaultNetwork reports whether name is a docker built-in network that
// must not be redefined in a compose file.
func isDefaultNetwork(name string) bool {
	return name == "bridge" || name == "host" || name == "none" ||
		strings.HasPrefix(name, "container:")
}

// captureNetworkDefs inspects every custom network the container is
// attached to and stores a recreatable definition on cfg.
func captureNetworkDefs(ctx context.Context, repo repository.ContainerRepository, cfg *model.ContainerConfig) {
	defs := map[string]model.NetworkDef{}
	for name := range cfg.Endpoints {
		if isDefaultNetwork(name) {
			continue
		}
		nw, err := repo.NetworkInspect(ctx, name)
		if err != nil {
			continue
		}
		def := model.NetworkDef{
			Name:       nw.Name,
			Driver:     nw.Driver,
			Internal:   nw.Internal,
			Attachable: nw.Attachable,
			EnableIPv6: nw.EnableIPv6,
			Options:    nw.Options,
			Labels:     nw.Labels,
		}
		for _, pool := range nw.IPAM.Config {
			def.Subnets = append(def.Subnets, model.IPAMSubnet{
				Subnet:  pool.Subnet,
				Gateway: pool.Gateway,
				IPRange: pool.IPRange,
			})
		}
		defs[name] = def
	}
	if len(defs) > 0 {
		cfg.NetworkDefs = defs
	}
}

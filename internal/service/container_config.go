package service

import (
	"strings"

	"backend/internal/model"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
)

func captureContainerConfig(inspect types.ContainerJSON) model.ContainerConfig {
	name := strings.TrimPrefix(inspect.Name, "/")

	var portBindings nat.PortMap
	if inspect.HostConfig != nil {
		portBindings = inspect.HostConfig.PortBindings
	}

	var networkMode container.NetworkMode
	var restartPolicy container.RestartPolicy
	var binds []string
	var autoRemove bool

	if inspect.HostConfig != nil {
		networkMode = inspect.HostConfig.NetworkMode
		restartPolicy = inspect.HostConfig.RestartPolicy
		binds = inspect.HostConfig.Binds
		autoRemove = inspect.HostConfig.AutoRemove
	}

	var cmd, entrypoint []string
	var env []string
	var labels map[string]string
	var image string

	if inspect.Config != nil {
		cmd = inspect.Config.Cmd
		entrypoint = inspect.Config.Entrypoint
		env = inspect.Config.Env
		labels = inspect.Config.Labels
		image = inspect.Config.Image
	}

	var networks []string
	if inspect.NetworkSettings != nil {
		for netName := range inspect.NetworkSettings.Networks {
			if string(networkMode) != netName {
				networks = append(networks, netName)
			}
		}
	}

	var mounts []types.MountPoint
	if inspect.Mounts != nil {
		mounts = inspect.Mounts
	}

	return model.ContainerConfig{
		Name:          name,
		Image:         image,
		Cmd:           cmd,
		Entrypoint:    entrypoint,
		Env:           env,
		Labels:        labels,
		Binds:         binds,
		Mounts:        mounts,
		PortBindings:  portBindings,
		NetworkMode:   networkMode,
		Networks:      networks,
		RestartPolicy: restartPolicy,
		AutoRemove:    autoRemove,
	}
}

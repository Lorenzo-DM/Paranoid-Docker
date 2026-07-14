package model

import (
	"time"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
)

type Port struct {
	HostPort      string `json:"host_port"`
	ContainerPort string `json:"container_port"`
	Protocol      string `json:"protocol"`
}

type Container struct {
	ID                   string    `json:"id"`
	ShortID              string    `json:"short_id"`
	Name                 string    `json:"name"`
	Image                string    `json:"image"`
	ImageID              string    `json:"image_id"`
	Status               string    `json:"status"`
	State                string    `json:"state"`
	CreatedAt            time.Time `json:"created_at"`
	Ports                []Port    `json:"ports"`
	UpdateAvailable      bool      `json:"update_available"`
	RemoteDigest         string    `json:"remote_digest,omitempty"`
	LocalDigest          string    `json:"local_digest,omitempty"`
	ComposeManagedFilter bool      `json:"-"`
}

// IPAMSubnet is one IPAM pool of a custom network.
type IPAMSubnet struct {
	Subnet  string
	Gateway string
	IPRange string
}

// NetworkDef captures enough of a custom network to recreate it.
type NetworkDef struct {
	Name       string
	Driver     string
	Internal   bool
	Attachable bool
	EnableIPv6 bool
	Subnets    []IPAMSubnet
	Options    map[string]string
	Labels     map[string]string
}

// ContainerConfig retains the raw docker inspect structs so nothing is
// dropped at capture time. Accessors below are nil-safe.
type ContainerConfig struct {
	ID          string
	Name        string
	Image       string
	Config      *container.Config
	HostConfig  *container.HostConfig
	Endpoints   map[string]*network.EndpointSettings
	Mounts      []dockertypes.MountPoint
	NetworkDefs map[string]NetworkDef
}

func (c ContainerConfig) Env() []string {
	if c.Config == nil {
		return nil
	}
	return c.Config.Env
}

func (c ContainerConfig) Cmd() []string {
	if c.Config == nil {
		return nil
	}
	return c.Config.Cmd
}

func (c ContainerConfig) Entrypoint() []string {
	if c.Config == nil {
		return nil
	}
	return c.Config.Entrypoint
}

func (c ContainerConfig) Labels() map[string]string {
	if c.Config == nil {
		return nil
	}
	return c.Config.Labels
}

func (c ContainerConfig) Binds() []string {
	if c.HostConfig == nil {
		return nil
	}
	return c.HostConfig.Binds
}

func (c ContainerConfig) PortBindings() nat.PortMap {
	if c.HostConfig == nil {
		return nil
	}
	return c.HostConfig.PortBindings
}

func (c ContainerConfig) NetworkMode() container.NetworkMode {
	if c.HostConfig == nil {
		return ""
	}
	return c.HostConfig.NetworkMode
}

func (c ContainerConfig) RestartPolicy() container.RestartPolicy {
	if c.HostConfig == nil {
		return container.RestartPolicy{}
	}
	return c.HostConfig.RestartPolicy
}

func (c ContainerConfig) AutoRemove() bool {
	if c.HostConfig == nil {
		return false
	}
	return c.HostConfig.AutoRemove
}

// Networks returns attached network names except the one already implied
// by NetworkMode.
func (c ContainerConfig) Networks() []string {
	var names []string
	for name := range c.Endpoints {
		if string(c.NetworkMode()) != name {
			names = append(names, name)
		}
	}
	return names
}

type RollbackFile struct {
	Filename      string    `json:"filename"`
	Path          string    `json:"path"`
	CreatedAt     time.Time `json:"created_at"`
	PreviousImage string    `json:"previous_image"`
}

type SavedImage struct {
	Filename  string    `json:"filename"`
	Path      string    `json:"path"`
	ImageRef  string    `json:"image_ref"`
	SizeBytes int64     `json:"size_bytes"`
	SavedAt   time.Time `json:"saved_at"`
}

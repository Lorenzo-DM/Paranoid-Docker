package model

import (
	"time"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
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

type ContainerConfig struct {
	Name          string
	Image         string
	Cmd           []string
	Entrypoint    []string
	Env           []string
	Labels        map[string]string
	Binds         []string
	Mounts        []dockertypes.MountPoint
	PortBindings  nat.PortMap
	NetworkMode   container.NetworkMode
	Networks      []string
	RestartPolicy container.RestartPolicy
	AutoRemove    bool
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

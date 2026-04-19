package repository

import (
	"context"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

type ContainerRepository interface {
	ListContainers(ctx context.Context) ([]types.Container, error)
	InspectContainer(ctx context.Context, id string) (types.ContainerJSON, error)
	InspectImage(ctx context.Context, imageID string) (types.ImageInspect, error)
	PullImage(ctx context.Context, imageRef string) (io.ReadCloser, error)
	StopContainer(ctx context.Context, id string, timeout *int) error
	RemoveContainer(ctx context.Context, id string) error
	CreateContainer(ctx context.Context, name string, cfg *container.Config, hostCfg *container.HostConfig, netCfg *network.NetworkingConfig) (string, error)
	StartContainer(ctx context.Context, id string) error
	ConnectNetwork(ctx context.Context, networkID, containerID string, endpointSettings *network.EndpointSettings) error
	ContainerLogs(ctx context.Context, id string, follow bool) (io.ReadCloser, error)
	SaveImage(ctx context.Context, imageID string) (io.ReadCloser, error)
	RemoveImage(ctx context.Context, imageID string) error
}

type containerRepository struct {
	cli *client.Client
}

func NewContainerRepository(cli *client.Client) ContainerRepository {
	return &containerRepository{cli: cli}
}

func (r *containerRepository) ListContainers(ctx context.Context) ([]types.Container, error) {
	return r.cli.ContainerList(ctx, container.ListOptions{
		All:     false,
		Filters: filters.NewArgs(),
	})
}

func (r *containerRepository) InspectContainer(ctx context.Context, id string) (types.ContainerJSON, error) {
	return r.cli.ContainerInspect(ctx, id)
}

func (r *containerRepository) InspectImage(ctx context.Context, imageID string) (types.ImageInspect, error) {
	inspect, _, err := r.cli.ImageInspectWithRaw(ctx, imageID)
	return inspect, err
}

func (r *containerRepository) PullImage(ctx context.Context, imageRef string) (io.ReadCloser, error) {
	return r.cli.ImagePull(ctx, imageRef, image.PullOptions{})
}

func (r *containerRepository) StopContainer(ctx context.Context, id string, timeout *int) error {
	return r.cli.ContainerStop(ctx, id, container.StopOptions{Timeout: timeout})
}

func (r *containerRepository) RemoveContainer(ctx context.Context, id string) error {
	return r.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true})
}

func (r *containerRepository) CreateContainer(ctx context.Context, name string, cfg *container.Config, hostCfg *container.HostConfig, netCfg *network.NetworkingConfig) (string, error) {
	resp, err := r.cli.ContainerCreate(ctx, cfg, hostCfg, netCfg, nil, name)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

func (r *containerRepository) StartContainer(ctx context.Context, id string) error {
	return r.cli.ContainerStart(ctx, id, container.StartOptions{})
}

func (r *containerRepository) ConnectNetwork(ctx context.Context, networkID, containerID string, endpointSettings *network.EndpointSettings) error {
	return r.cli.NetworkConnect(ctx, networkID, containerID, endpointSettings)
}

func (r *containerRepository) ContainerLogs(ctx context.Context, id string, follow bool) (io.ReadCloser, error) {
	return r.cli.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     follow,
		Timestamps: false,
		Tail:       "200",
	})
}

func (r *containerRepository) SaveImage(ctx context.Context, imageID string) (io.ReadCloser, error) {
	return r.cli.ImageSave(ctx, []string{imageID})
}

func (r *containerRepository) RemoveImage(ctx context.Context, imageID string) error {
	_, err := r.cli.ImageRemove(ctx, imageID, image.RemoveOptions{PruneChildren: true})
	return err
}

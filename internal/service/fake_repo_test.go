package service

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
)

// fakeRepo implements repository.ContainerRepository with programmable
// returns and call recording.
type fakeRepo struct {
	mu    sync.Mutex
	calls []string

	containers    []types.Container
	listErr       error
	inspects      map[string]types.ContainerJSON
	inspectErr    error
	imageInspects map[string]types.ImageInspect
	imageErr      error
	pullBody      string
	pullErr       error
	stopErr       error
	removeErr     error
	createID      string
	createErr     error
	startErr      error
	connectErr    error
	logsBody      string
	logsErr       error
	networks      map[string]network.Inspect
	networkErr    error
	saveBody      string
	saveErr       error
	removeImgErr  error

	createdName    string
	createdCfg     *container.Config
	createdHostCfg *container.HostConfig
	createdNetCfg  *network.NetworkingConfig
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		inspects:      map[string]types.ContainerJSON{},
		imageInspects: map[string]types.ImageInspect{},
		networks:      map[string]network.Inspect{},
		createID:      "new-container-id",
	}
}

func (f *fakeRepo) record(call string) {
	f.mu.Lock()
	f.calls = append(f.calls, call)
	f.mu.Unlock()
}

func (f *fakeRepo) callNames() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	names := make([]string, len(f.calls))
	copy(names, f.calls)
	return names
}

func (f *fakeRepo) ListContainers(ctx context.Context) ([]types.Container, error) {
	f.record("ListContainers")
	return f.containers, f.listErr
}

func (f *fakeRepo) InspectContainer(ctx context.Context, id string) (types.ContainerJSON, error) {
	f.record("InspectContainer:" + id)
	if f.inspectErr != nil {
		return types.ContainerJSON{}, f.inspectErr
	}
	if insp, ok := f.inspects[id]; ok {
		return insp, nil
	}
	return types.ContainerJSON{}, fmt.Errorf("no such container: %s", id)
}

func (f *fakeRepo) InspectImage(ctx context.Context, imageID string) (types.ImageInspect, error) {
	f.record("InspectImage:" + imageID)
	if f.imageErr != nil {
		return types.ImageInspect{}, f.imageErr
	}
	if insp, ok := f.imageInspects[imageID]; ok {
		return insp, nil
	}
	return types.ImageInspect{}, fmt.Errorf("no such image: %s", imageID)
}

func (f *fakeRepo) PullImage(ctx context.Context, imageRef string) (io.ReadCloser, error) {
	f.record("PullImage:" + imageRef)
	if f.pullErr != nil {
		return nil, f.pullErr
	}
	return io.NopCloser(strings.NewReader(f.pullBody)), nil
}

func (f *fakeRepo) StopContainer(ctx context.Context, id string, timeout *int) error {
	f.record("StopContainer:" + id)
	return f.stopErr
}

func (f *fakeRepo) RemoveContainer(ctx context.Context, id string) error {
	f.record("RemoveContainer:" + id)
	return f.removeErr
}

func (f *fakeRepo) CreateContainer(ctx context.Context, name string, cfg *container.Config, hostCfg *container.HostConfig, netCfg *network.NetworkingConfig) (string, error) {
	f.record("CreateContainer:" + name)
	f.createdName = name
	f.createdCfg = cfg
	f.createdHostCfg = hostCfg
	f.createdNetCfg = netCfg
	if f.createErr != nil {
		return "", f.createErr
	}
	return f.createID, nil
}

func (f *fakeRepo) StartContainer(ctx context.Context, id string) error {
	f.record("StartContainer:" + id)
	return f.startErr
}

func (f *fakeRepo) ConnectNetwork(ctx context.Context, networkID, containerID string, endpointSettings *network.EndpointSettings) error {
	f.record("ConnectNetwork:" + networkID)
	return f.connectErr
}

func (f *fakeRepo) NetworkInspect(ctx context.Context, nameOrID string) (network.Inspect, error) {
	f.record("NetworkInspect:" + nameOrID)
	if f.networkErr != nil {
		return network.Inspect{}, f.networkErr
	}
	if nw, ok := f.networks[nameOrID]; ok {
		return nw, nil
	}
	return network.Inspect{}, fmt.Errorf("no such network: %s", nameOrID)
}

func (f *fakeRepo) ContainerLogs(ctx context.Context, id string, follow bool) (io.ReadCloser, error) {
	f.record("ContainerLogs:" + id)
	if f.logsErr != nil {
		return nil, f.logsErr
	}
	return io.NopCloser(strings.NewReader(f.logsBody)), nil
}

func (f *fakeRepo) SaveImage(ctx context.Context, imageID string) (io.ReadCloser, error) {
	f.record("SaveImage:" + imageID)
	if f.saveErr != nil {
		return nil, f.saveErr
	}
	return io.NopCloser(strings.NewReader(f.saveBody)), nil
}

func (f *fakeRepo) RemoveImage(ctx context.Context, imageID string) error {
	f.record("RemoveImage:" + imageID)
	return f.removeImgErr
}

// fakeRunner implements CommandRunner with canned output.
type fakeRunner struct {
	mu       sync.Mutex
	commands [][]string

	output    []byte
	outputErr error
	streamOut string
	streamErr error
}

func (f *fakeRunner) record(name string, args []string) {
	f.mu.Lock()
	f.commands = append(f.commands, append([]string{name}, args...))
	f.mu.Unlock()
}

func (f *fakeRunner) Output(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	f.record(name, args)
	return f.output, f.outputErr
}

func (f *fakeRunner) Stream(ctx context.Context, dir string, stdout, stderr io.Writer, name string, args ...string) error {
	f.record(name, args)
	if f.streamOut != "" {
		_, _ = io.WriteString(stdout, f.streamOut)
	}
	return f.streamErr
}

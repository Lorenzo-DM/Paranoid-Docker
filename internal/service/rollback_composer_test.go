package service

import (
	"os"
	"path/filepath"
	"testing"

	"backend/internal/model"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"gopkg.in/yaml.v3"
)

func testContainerConfig() model.ContainerConfig {
	return model.ContainerConfig{
		ID:    "0123456789abcdef0123",
		Name:  "myapp",
		Image: "nginx:1.25",
		Config: &container.Config{
			Image:  "nginx:1.25",
			Env:    []string{"FOO=bar", "SECRET=x"},
			Labels: map[string]string{"custom.label": "v", "com.docker.compose.project": "p"},
		},
		HostConfig: &container.HostConfig{
			Binds: []string{"/host/data:/data"},
			PortBindings: nat.PortMap{
				"80/tcp": []nat.PortBinding{{HostPort: "8080"}},
			},
			NetworkMode:   "bridge",
			RestartPolicy: container.RestartPolicy{Name: "unless-stopped"},
		},
		Endpoints: map[string]*network.EndpointSettings{
			"mynet": {},
		},
		Mounts: []types.MountPoint{
			{Type: "volume", Name: "appvol", Destination: "/var/lib/app"},
		},
	}
}

func TestWriteRollbackComposeRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	w := NewRollbackWriter(tmp, &fakeRunner{})

	path, err := w.WriteRollbackCompose(testContainerConfig(), "nginx@sha256:abc", true)
	if err != nil {
		t.Fatalf("WriteRollbackCompose: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read rollback file: %v", err)
	}

	var cf composeFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		t.Fatalf("unmarshal rollback yaml: %v", err)
	}

	svc, ok := cf.Services["myapp"]
	if !ok {
		t.Fatalf("services = %v, want myapp", cf.Services)
	}
	if svc.Image != "nginx@sha256:abc" {
		t.Errorf("image = %q, want pinned digest", svc.Image)
	}
	if svc.Restart != "unless-stopped" {
		t.Errorf("restart = %q", svc.Restart)
	}
	if len(svc.Ports) != 1 || svc.Ports[0] != "8080:80/tcp" {
		t.Errorf("ports = %v", svc.Ports)
	}
	wantVols := map[string]bool{"/host/data:/data": true, "appvol:/var/lib/app": true}
	if len(svc.Volumes) != 2 || !wantVols[svc.Volumes[0]] || !wantVols[svc.Volumes[1]] {
		t.Errorf("volumes = %v", svc.Volumes)
	}
	if len(svc.Environment) != 2 {
		t.Errorf("environment = %v, want included env", svc.Environment)
	}
	if _, ok := svc.Labels["com.docker.compose.project"]; ok {
		t.Error("compose-internal label leaked into rollback")
	}
	if svc.Labels["custom.label"] != "v" {
		t.Errorf("labels = %v", svc.Labels)
	}
	if _, ok := cf.Networks["mynet"]; !ok {
		t.Errorf("networks = %v, want mynet", cf.Networks)
	}
	if _, ok := cf.Volumes["appvol"]; !ok {
		t.Errorf("volumes section = %v, want appvol", cf.Volumes)
	}
}

func TestWriteRollbackComposeExcludesEnv(t *testing.T) {
	tmp := t.TempDir()
	w := NewRollbackWriter(tmp, &fakeRunner{})

	path, err := w.WriteRollbackCompose(testContainerConfig(), "", false)
	if err != nil {
		t.Fatalf("WriteRollbackCompose: %v", err)
	}
	data, _ := os.ReadFile(path)
	var cf composeFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		t.Fatal(err)
	}
	if env := cf.Services["myapp"].Environment; len(env) != 0 {
		t.Errorf("environment = %v, want empty", env)
	}
	if img := cf.Services["myapp"].Image; img != "nginx:1.25" {
		t.Errorf("image = %q, want original ref when no digest", img)
	}
}

func TestListRollbacks(t *testing.T) {
	tmp := t.TempDir()
	w := NewRollbackWriter(tmp, &fakeRunner{})

	if files, err := w.ListRollbacks("nothing"); err != nil || len(files) != 0 {
		t.Fatalf("empty list = %v, %v", files, err)
	}

	if _, err := w.WriteRollbackCompose(testContainerConfig(), "nginx@sha256:abc", false); err != nil {
		t.Fatal(err)
	}

	files, err := w.ListRollbacks("myapp")
	if err != nil {
		t.Fatalf("ListRollbacks: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 rollback, got %d", len(files))
	}
	if files[0].PreviousImage != "nginx@sha256:abc" {
		t.Errorf("PreviousImage = %q", files[0].PreviousImage)
	}
	if filepath.Dir(files[0].Path) != filepath.Join(tmp, "myapp") {
		t.Errorf("Path = %q not under base dir", files[0].Path)
	}
}

func TestWriteAndListStackRollbacksInspectMode(t *testing.T) {
	tmp := t.TempDir()
	w := NewRollbackWriter(tmp, &fakeRunner{})
	repo := newFakeRepo()
	repo.inspects["c1"] = types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			Name: "/web-1",
			HostConfig: &container.HostConfig{
				NetworkMode:   "mynet",
				RestartPolicy: container.RestartPolicy{Name: "always"},
			},
		},
		Config: &container.Config{Image: "nginx:1.25"},
	}

	stack := model.ComposeStack{
		Name: "mystack",
		Services: []model.ComposeService{
			{Name: "web", ContainerID: "c1", Image: "nginx:1.25", LocalDigest: "sha256:abc"},
		},
	}

	dir, err := w.WriteStackRollback(t.Context(), stack, repo, false, "inspect")
	if err != nil {
		t.Fatalf("WriteStackRollback: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "docker-compose.yaml"))
	if err != nil {
		t.Fatalf("read rollback: %v", err)
	}
	var cf composeFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		t.Fatal(err)
	}
	if cf.Services["web"].Image != "nginx@sha256:abc" {
		t.Errorf("image = %q, want pinned", cf.Services["web"].Image)
	}

	files, err := w.ListStackRollbacks("mystack")
	if err != nil {
		t.Fatalf("ListStackRollbacks: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 rollback, got %d", len(files))
	}
}

func TestWriteStackRollbackComposeMode(t *testing.T) {
	tmp := t.TempDir()
	runner := &fakeRunner{output: []byte("services:\n  web:\n    image: nginx:1.25\n")}
	w := NewRollbackWriter(tmp, runner)

	stack := model.ComposeStack{
		Name:        "mystack",
		ConfigFiles: []string{"/stacks/mystack/docker-compose.yaml"},
		Services: []model.ComposeService{
			{Name: "web", ContainerID: "c1", Image: "nginx:1.25", LocalDigest: "sha256:abc"},
		},
	}

	dir, err := w.WriteStackRollback(t.Context(), stack, newFakeRepo(), false, "compose")
	if err != nil {
		t.Fatalf("WriteStackRollback: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "docker-compose.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var cf composeFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		t.Fatal(err)
	}
	if cf.Services["web"].Image != "nginx@sha256:abc" {
		t.Errorf("image = %q, want pinned via patchComposeImages", cf.Services["web"].Image)
	}
	if len(runner.commands) != 1 || runner.commands[0][0] != "docker" || runner.commands[0][1] != "compose" {
		t.Errorf("runner commands = %v", runner.commands)
	}
}

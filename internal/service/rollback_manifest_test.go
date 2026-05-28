package service

import (
	"reflect"
	"testing"
	"time"

	"backend/internal/model"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/go-connections/nat"
)

func TestMaskEnv(t *testing.T) {
	got := MaskEnv([]string{"TOKEN=abc", "MODE=prod"})
	want := map[string]string{
		"TOKEN": "********",
		"MODE":  "********",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MaskEnv() = %#v, want %#v", got, want)
	}
}

func TestMaskEnvUsesWholeEntryWhenMissingEquals(t *testing.T) {
	got := MaskEnv([]string{"TOKEN", "MODE=prod=blue"})
	want := map[string]string{
		"TOKEN": "********",
		"MODE":  "********",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MaskEnv() = %#v, want %#v", got, want)
	}
}

func TestBuildContainerRollbackManifest(t *testing.T) {
	now := time.Date(2026, 5, 26, 12, 30, 0, 0, time.UTC)
	cfg := model.ContainerConfig{
		Name:  "api",
		Image: "example/api:new",
		Env:   []string{"TOKEN=abc", "MODE=prod"},
	}

	manifest := BuildContainerRollbackManifest(cfg, "example/api:old", now)

	if manifest.Version != 1 {
		t.Fatalf("Version = %d, want 1", manifest.Version)
	}
	if manifest.TargetType != model.RollbackTargetContainer {
		t.Fatalf("TargetType = %q, want %q", manifest.TargetType, model.RollbackTargetContainer)
	}
	if manifest.TargetName != cfg.Name {
		t.Fatalf("TargetName = %q, want %q", manifest.TargetName, cfg.Name)
	}
	if !manifest.CreatedAt.Equal(now) {
		t.Fatalf("CreatedAt = %s, want %s", manifest.CreatedAt, now)
	}
	if manifest.SourceMode != model.RollbackSourceInspect {
		t.Fatalf("SourceMode = %q, want %q", manifest.SourceMode, model.RollbackSourceInspect)
	}
	if len(manifest.Items) != 1 {
		t.Fatalf("len(Items) = %d, want 1", len(manifest.Items))
	}

	item := manifest.Items[0]
	if item.Name != cfg.Name {
		t.Fatalf("item.Name = %q, want %q", item.Name, cfg.Name)
	}
	if item.CurrentImage != cfg.Image {
		t.Fatalf("CurrentImage = %q, want %q", item.CurrentImage, cfg.Image)
	}
	if item.RollbackImage != "example/api:old" {
		t.Fatalf("RollbackImage = %q, want example/api:old", item.RollbackImage)
	}
	if !reflect.DeepEqual(item.Config.Env, cfg.Env) {
		t.Fatalf("Config.Env = %#v, want %#v", item.Config.Env, cfg.Env)
	}
	wantMasked := map[string]string{"TOKEN": "********", "MODE": "********"}
	if !reflect.DeepEqual(item.EnvMasked, wantMasked) {
		t.Fatalf("EnvMasked = %#v, want %#v", item.EnvMasked, wantMasked)
	}
}

func TestBuildContainerRollbackManifestFallsBackToCurrentImage(t *testing.T) {
	cfg := model.ContainerConfig{Name: "api", Image: "example/api:new"}

	manifest := BuildContainerRollbackManifest(cfg, "", time.Time{})

	if got := manifest.Items[0].RollbackImage; got != cfg.Image {
		t.Fatalf("RollbackImage = %q, want %q", got, cfg.Image)
	}
}

func TestBuildContainerRollbackManifestSnapshotsConfig(t *testing.T) {
	port := nat.Port("8080/tcp")
	cfg := model.ContainerConfig{
		Name:       "api",
		Image:      "example/api:new",
		Cmd:        []string{"serve"},
		Entrypoint: []string{"/entrypoint.sh"},
		Env:        []string{"TOKEN=abc"},
		Labels:     map[string]string{"com.example.role": "api"},
		Binds:      []string{"/host:/container"},
		Mounts:     []dockertypes.MountPoint{{Destination: "/data"}},
		Networks:   []string{"frontend"},
		PortBindings: nat.PortMap{
			port: []nat.PortBinding{{HostIP: "127.0.0.1", HostPort: "8080"}},
		},
	}

	manifest := BuildContainerRollbackManifest(cfg, "example/api:old", time.Time{})

	cfg.Cmd[0] = "mutated"
	cfg.Entrypoint[0] = "/mutated.sh"
	cfg.Env[0] = "TOKEN=mutated"
	cfg.Labels["com.example.role"] = "mutated"
	cfg.Binds[0] = "/mutated:/container"
	cfg.Mounts[0].Destination = "/mutated"
	cfg.Networks[0] = "mutated"
	cfg.PortBindings[port][0].HostPort = "9090"

	item := manifest.Items[0]
	if got := item.Config.Cmd[0]; got != "serve" {
		t.Fatalf("Config.Cmd[0] = %q, want serve", got)
	}
	if got := item.Config.Entrypoint[0]; got != "/entrypoint.sh" {
		t.Fatalf("Config.Entrypoint[0] = %q, want /entrypoint.sh", got)
	}
	if got := item.Config.Env[0]; got != "TOKEN=abc" {
		t.Fatalf("Config.Env[0] = %q, want TOKEN=abc", got)
	}
	if got := item.Config.Labels["com.example.role"]; got != "api" {
		t.Fatalf("Config.Labels role = %q, want api", got)
	}
	if got := item.Config.Binds[0]; got != "/host:/container" {
		t.Fatalf("Config.Binds[0] = %q, want /host:/container", got)
	}
	if got := item.Config.Mounts[0].Destination; got != "/data" {
		t.Fatalf("Config.Mounts[0].Destination = %q, want /data", got)
	}
	if got := item.Config.Networks[0]; got != "frontend" {
		t.Fatalf("Config.Networks[0] = %q, want frontend", got)
	}
	if got := item.Config.PortBindings[port][0].HostPort; got != "8080" {
		t.Fatalf("Config.PortBindings host port = %q, want 8080", got)
	}
	if !reflect.DeepEqual(item.EnvMasked, map[string]string{"TOKEN": "********"}) {
		t.Fatalf("EnvMasked = %#v, want original masked TOKEN", item.EnvMasked)
	}
}

func TestBuildStackRollbackManifest(t *testing.T) {
	now := time.Date(2026, 5, 26, 13, 0, 0, 0, time.UTC)
	stack := model.ComposeStack{
		Name:        "prod",
		ConfigFiles: []string{"/srv/prod/compose.yaml"},
		WorkingDir:  "/srv/prod",
		Services: []model.ComposeService{
			{Name: "api", ContainerID: "api-container-id"},
			{Name: "worker", ContainerID: "worker-container-id"},
		},
	}
	configs := []model.ContainerConfig{
		{
			Name:   "prod-api-1",
			Image:  "example/api:new",
			Env:    []string{"TOKEN=abc"},
			Labels: map[string]string{"com.docker.compose.service": "api"},
		},
		{
			Name:  "worker",
			Image: "example/worker:new",
			Env:   []string{"MODE=prod"},
		},
	}
	rollbackImages := map[string]string{
		"api": "example/api:old",
	}

	manifest := BuildStackRollbackManifest(stack, configs, rollbackImages, model.RollbackSourceCompose, now)

	if manifest.Version != 1 {
		t.Fatalf("Version = %d, want 1", manifest.Version)
	}
	if manifest.TargetType != model.RollbackTargetStack {
		t.Fatalf("TargetType = %q, want %q", manifest.TargetType, model.RollbackTargetStack)
	}
	if manifest.TargetName != stack.Name {
		t.Fatalf("TargetName = %q, want %q", manifest.TargetName, stack.Name)
	}
	if !manifest.CreatedAt.Equal(now) {
		t.Fatalf("CreatedAt = %s, want %s", manifest.CreatedAt, now)
	}
	if manifest.SourceMode != model.RollbackSourceCompose {
		t.Fatalf("SourceMode = %q, want %q", manifest.SourceMode, model.RollbackSourceCompose)
	}
	if !reflect.DeepEqual(manifest.ComposeFiles, stack.ConfigFiles) {
		t.Fatalf("ComposeFiles = %#v, want %#v", manifest.ComposeFiles, stack.ConfigFiles)
	}
	if manifest.WorkingDir != stack.WorkingDir {
		t.Fatalf("WorkingDir = %q, want %q", manifest.WorkingDir, stack.WorkingDir)
	}
	if len(manifest.Items) != 2 {
		t.Fatalf("len(Items) = %d, want 2", len(manifest.Items))
	}

	api := manifest.Items[0]
	if api.Name != "prod-api-1" {
		t.Fatalf("api.Name = %q, want prod-api-1", api.Name)
	}
	if api.ServiceName != "api" {
		t.Fatalf("api.ServiceName = %q, want api", api.ServiceName)
	}
	if api.ContainerID != "api-container-id" {
		t.Fatalf("api.ContainerID = %q, want api-container-id", api.ContainerID)
	}
	if api.RollbackImage != "example/api:old" {
		t.Fatalf("api.RollbackImage = %q, want example/api:old", api.RollbackImage)
	}
	if !reflect.DeepEqual(api.Config.Env, configs[0].Env) {
		t.Fatalf("api.Config.Env = %#v, want %#v", api.Config.Env, configs[0].Env)
	}
	if !reflect.DeepEqual(api.EnvMasked, map[string]string{"TOKEN": "********"}) {
		t.Fatalf("api.EnvMasked = %#v, want masked TOKEN", api.EnvMasked)
	}

	worker := manifest.Items[1]
	if worker.ServiceName != "worker" {
		t.Fatalf("worker.ServiceName = %q, want worker", worker.ServiceName)
	}
	if worker.ContainerID != "worker-container-id" {
		t.Fatalf("worker.ContainerID = %q, want worker-container-id", worker.ContainerID)
	}
	if worker.RollbackImage != "example/worker:new" {
		t.Fatalf("worker.RollbackImage = %q, want fallback current image", worker.RollbackImage)
	}
}

func TestBuildStackRollbackManifestDefaultsSourceModeToInspect(t *testing.T) {
	manifest := BuildStackRollbackManifest(
		model.ComposeStack{Name: "prod"},
		[]model.ContainerConfig{{Name: "api", Image: "example/api:new"}},
		nil,
		"",
		time.Time{},
	)

	if manifest.SourceMode != model.RollbackSourceInspect {
		t.Fatalf("SourceMode = %q, want %q", manifest.SourceMode, model.RollbackSourceInspect)
	}
}

func TestBuildStackRollbackManifestUsesInspectWhenComposeFilesUnavailable(t *testing.T) {
	manifest := BuildStackRollbackManifest(
		model.ComposeStack{Name: "prod"},
		[]model.ContainerConfig{{Name: "api", Image: "example/api:new"}},
		nil,
		model.RollbackSourceCompose,
		time.Time{},
	)

	if manifest.SourceMode != model.RollbackSourceInspect {
		t.Fatalf("SourceMode = %q, want %q", manifest.SourceMode, model.RollbackSourceInspect)
	}
}

func TestBuildStackRollbackManifestPreservesUnmatchedComposeServiceLabel(t *testing.T) {
	configs := []model.ContainerConfig{
		{
			Name:   "prod-api-1",
			Image:  "example/api:new",
			Labels: map[string]string{"com.docker.compose.service": "api"},
		},
	}
	rollbackImages := map[string]string{"api": "example/api:old"}

	manifest := BuildStackRollbackManifest(
		model.ComposeStack{Name: "prod"},
		configs,
		rollbackImages,
		model.RollbackSourceInspect,
		time.Time{},
	)

	item := manifest.Items[0]
	if item.ServiceName != "api" {
		t.Fatalf("ServiceName = %q, want api", item.ServiceName)
	}
	if item.ContainerID != "" {
		t.Fatalf("ContainerID = %q, want empty", item.ContainerID)
	}
	if item.RollbackImage != "example/api:old" {
		t.Fatalf("RollbackImage = %q, want rollback image by service label", item.RollbackImage)
	}
}

func TestBuildStackRollbackManifestFallsBackToRollbackImageByConfigName(t *testing.T) {
	configs := []model.ContainerConfig{
		{
			Name:   "api-container",
			Image:  "example/api:new",
			Labels: map[string]string{"com.docker.compose.service": "api"},
		},
	}
	rollbackImages := map[string]string{"api-container": "example/api:old"}

	manifest := BuildStackRollbackManifest(
		model.ComposeStack{Name: "prod"},
		configs,
		rollbackImages,
		model.RollbackSourceInspect,
		time.Time{},
	)

	if got := manifest.Items[0].ServiceName; got != "api" {
		t.Fatalf("ServiceName = %q, want api", got)
	}
	if got := manifest.Items[0].RollbackImage; got != "example/api:old" {
		t.Fatalf("RollbackImage = %q, want rollback image by config name", got)
	}
}

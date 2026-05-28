package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"backend/internal/model"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"gopkg.in/yaml.v3"
)

func TestRenderRollbackComposeSafePreviewExcludesMaskedEnv(t *testing.T) {
	manifest := rollbackTestManifest(time.Date(2026, 5, 26, 10, 11, 12, 0, time.UTC))

	data, err := RenderRollbackCompose(manifest, false)
	if err != nil {
		t.Fatalf("RenderRollbackCompose() error = %v", err)
	}

	raw := string(data)
	for _, want := range []string{
		"image: example/api@sha256:old",
		"container_name: api",
		"restart: unless-stopped",
		"- 127.0.0.1:8080:80/tcp",
		"- app_data:/var/lib/app",
		"external_net:",
		"external: true",
		"com.example.owner: platform",
	} {
		if !strings.Contains(raw, want) {
			t.Fatalf("rendered YAML missing %q:\n%s", want, raw)
		}
	}

	for _, notWant := range []string{
		"environment:",
		"SECRET_TOKEN=super-secret",
		"PUBLIC_MODE=prod",
		"com.docker.compose.project",
	} {
		if strings.Contains(raw, notWant) {
			t.Fatalf("rendered safe YAML contains %q:\n%s", notWant, raw)
		}
	}

	var parsed composeFile
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("rendered YAML did not unmarshal: %v\n%s", err, raw)
	}
	if _, ok := parsed.Volumes["app_data"]; !ok {
		t.Fatalf("named volume app_data was not declared external: %#v", parsed.Volumes)
	}
	if _, ok := parsed.Networks["external_net"]; !ok {
		t.Fatalf("network external_net was not declared external: %#v", parsed.Networks)
	}
}

func TestRenderRollbackComposeIncludesEnvWhenRequested(t *testing.T) {
	manifest := rollbackTestManifest(time.Date(2026, 5, 26, 10, 11, 12, 0, time.UTC))

	data, err := RenderRollbackCompose(manifest, true)
	if err != nil {
		t.Fatalf("RenderRollbackCompose() error = %v", err)
	}

	raw := string(data)
	for _, want := range []string{
		"environment:",
		"- SECRET_TOKEN=super-secret",
		"- PUBLIC_MODE=prod",
	} {
		if !strings.Contains(raw, want) {
			t.Fatalf("rendered YAML missing %q:\n%s", want, raw)
		}
	}
}

func TestWriteRollbackSnapshotCreatesManifestAndSafeYAML(t *testing.T) {
	createdAt := time.Date(2026, 5, 26, 10, 11, 12, 0, time.UTC)
	manifest := rollbackTestManifest(createdAt)
	baseDir := t.TempDir()

	snapshotDir, err := WriteRollbackSnapshot(baseDir, manifest)
	if err != nil {
		t.Fatalf("WriteRollbackSnapshot() error = %v", err)
	}

	wantDir := filepath.Join(baseDir, "api", "2026-05-26T10-11-12")
	if snapshotDir != wantDir {
		t.Fatalf("snapshotDir = %q, want %q", snapshotDir, wantDir)
	}

	manifestPath := filepath.Join(snapshotDir, "rollback-manifest.json")
	yamlPath := filepath.Join(snapshotDir, "docker-compose.rollback.yaml")
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("manifest file was not created: %v", err)
	}
	yamlData, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatalf("rollback YAML was not created: %v", err)
	}
	if strings.Contains(string(yamlData), "SECRET_TOKEN=super-secret") {
		t.Fatalf("snapshot YAML includes raw env:\n%s", string(yamlData))
	}

	readManifest, err := ReadRollbackManifest(snapshotDir)
	if err != nil {
		t.Fatalf("ReadRollbackManifest(snapshotDir) error = %v", err)
	}
	if !readManifest.CreatedAt.Equal(createdAt) {
		t.Fatalf("CreatedAt = %s, want %s", readManifest.CreatedAt, createdAt)
	}

	readManifest, err = ReadRollbackManifest(manifestPath)
	if err != nil {
		t.Fatalf("ReadRollbackManifest(manifestPath) error = %v", err)
	}
	if readManifest.TargetName != "api" {
		t.Fatalf("TargetName = %q, want api", readManifest.TargetName)
	}

	files, err := ListRollbackSnapshots(baseDir, "api")
	if err != nil {
		t.Fatalf("ListRollbackSnapshots() error = %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("len(files) = %d, want 1: %#v", len(files), files)
	}
	if files[0].Filename != filepath.Join("2026-05-26T10-11-12", "docker-compose.rollback.yaml") {
		t.Fatalf("Filename = %q", files[0].Filename)
	}
	if files[0].Path != yamlPath {
		t.Fatalf("Path = %q, want %q", files[0].Path, yamlPath)
	}
	if files[0].PreviousImage != "example/api@sha256:old" {
		t.Fatalf("PreviousImage = %q", files[0].PreviousImage)
	}
}

func rollbackTestManifest(createdAt time.Time) model.RollbackManifest {
	return model.RollbackManifest{
		Version:    1,
		TargetType: model.RollbackTargetContainer,
		TargetName: "api",
		CreatedAt:  createdAt,
		SourceMode: model.RollbackSourceInspect,
		Items: []model.RollbackItem{
			{
				Name:          "api",
				CurrentImage:  "example/api:latest",
				RollbackImage: "example/api@sha256:old",
				Config: model.ContainerConfig{
					Name:  "api",
					Image: "example/api:latest",
					Env: []string{
						"SECRET_TOKEN=super-secret",
						"PUBLIC_MODE=prod",
					},
					Labels: map[string]string{
						"com.example.owner":          "platform",
						"com.docker.compose.project": "ignored",
					},
					Mounts: []dockertypes.MountPoint{
						{
							Type:        "volume",
							Name:        "app_data",
							Destination: "/var/lib/app",
						},
					},
					PortBindings: nat.PortMap{
						nat.Port("80/tcp"): []nat.PortBinding{
							{HostIP: "127.0.0.1", HostPort: "8080"},
						},
					},
					Networks: []string{"external_net"},
					RestartPolicy: container.RestartPolicy{
						Name: "unless-stopped",
					},
				},
				EnvMasked: MaskEnv([]string{"SECRET_TOKEN=super-secret", "PUBLIC_MODE=prod"}),
			},
		},
	}
}

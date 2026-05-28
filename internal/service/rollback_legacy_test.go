package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"backend/internal/model"
	"backend/internal/repository"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
)

type fakeLegacyContainerRepo struct {
	repository.ContainerRepository
	InspectFunc func(ctx context.Context, id string) (types.ContainerJSON, error)
}

func (f *fakeLegacyContainerRepo) InspectContainer(ctx context.Context, id string) (types.ContainerJSON, error) {
	if f.InspectFunc != nil {
		return f.InspectFunc(ctx, id)
	}
	return types.ContainerJSON{}, nil
}

func TestWriteRollbackComposeLegacy(t *testing.T) {
	// Temp directory for baseDir/rollbacks
	tempDir, err := os.MkdirTemp("", "rollbacks-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Temporarily change working directory to tempDir so relative paths work, or mock/refactor.
	// Actually, the original WriteRollbackCompose writes to "rollbacks/".
	// Let's change cwd to tempDir for the duration of this test.
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	cfg := model.ContainerConfig{
		Name:  "test-legacy-container",
		Image: "example/api:new",
		Env:   []string{"TOKEN=123"},
	}

	path, err := WriteRollbackCompose(cfg, "example/api:old", true)
	if err != nil {
		t.Fatalf("WriteRollbackCompose() error = %v", err)
	}

	// Verify path matches rollbacks/test-legacy-container/<timestamp>/docker-compose.rollback.yaml
	if !filepath.IsLocal(path) && !filepath.IsAbs(path) {
		t.Fatalf("invalid path: %q", path)
	}

	// Expect both manifest and yaml in the dir
	dir := filepath.Dir(path)
	manifestPath := filepath.Join(dir, "rollback-manifest.json")
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("manifest not created: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("yaml not created: %v", err)
	}

	// Read manifest and verify
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest model.RollbackManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}

	if manifest.TargetName != "test-legacy-container" {
		t.Fatalf("target name = %q, want %q", manifest.TargetName, "test-legacy-container")
	}
	if len(manifest.Items) != 1 || manifest.Items[0].RollbackImage != "example/api:old" {
		t.Fatalf("manifest items mismatch: %#v", manifest.Items)
	}
}

func TestWriteStackRollbackLegacy(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rollbacks-stack-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	stack := model.ComposeStack{
		Name: "test-legacy-stack",
		Services: []model.ComposeService{
			{Name: "web", ContainerID: "web-id", Image: "example/web:latest", LocalDigest: "sha256:web-old"},
		},
	}

	fakeRepo := &fakeLegacyContainerRepo{
		InspectFunc: func(ctx context.Context, id string) (types.ContainerJSON, error) {
			if id == "web-id" {
				return types.ContainerJSON{
					ContainerJSONBase: &types.ContainerJSONBase{
						Name: "/test-legacy-stack-web-1",
					},
					Config: &container.Config{
						Image: "example/web:latest",
						Env:   []string{"PORT=80"},
						Labels: map[string]string{
							"com.docker.compose.service": "web",
						},
					},
				}, nil
			}
			return types.ContainerJSON{}, nil
		},
	}

	path, err := WriteStackRollback(context.Background(), stack, fakeRepo, true, "inspect")
	if err != nil {
		t.Fatalf("WriteStackRollback() error = %v", err)
	}

	// Verify files under the returned path
	manifestPath := filepath.Join(path, "rollback-manifest.json")
	yamlPath := filepath.Join(path, "docker-compose.rollback.yaml")

	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("manifest not created under stack snapshot dir: %v", err)
	}
	if _, err := os.Stat(yamlPath); err != nil {
		t.Fatalf("yaml not created under stack snapshot dir: %v", err)
	}

	// Read and verify manifest
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read stack manifest: %v", err)
	}
	var manifest model.RollbackManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("unmarshal stack manifest: %v", err)
	}

	if manifest.TargetName != "test-legacy-stack" {
		t.Fatalf("target name = %q, want %q", manifest.TargetName, "test-legacy-stack")
	}
	if len(manifest.Items) != 1 || manifest.Items[0].RollbackImage != "example/web@sha256:web-old" {
		t.Fatalf("manifest items mismatch: %#v", manifest.Items)
	}
}

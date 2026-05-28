package service

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"backend/internal/model"
	"backend/internal/repository"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
)

type fakeExecutionRepo struct {
	repository.ContainerRepository
	InspectFunc func(ctx context.Context, id string) (types.ContainerJSON, error)
	PullFunc    func(ctx context.Context, ref string) (io.ReadCloser, error)
	StopFunc    func(ctx context.Context, id string, timeout *int) error
	RemoveFunc  func(ctx context.Context, id string) error
	CreateFunc  func(ctx context.Context, name string, cfg *container.Config, hostCfg *container.HostConfig, netCfg *network.NetworkingConfig) (string, error)
	StartFunc   func(ctx context.Context, id string) error

	CreatedName string
	CreatedCfg  *container.Config
}

func (f *fakeExecutionRepo) InspectContainer(ctx context.Context, id string) (types.ContainerJSON, error) {
	if f.InspectFunc != nil {
		return f.InspectFunc(ctx, id)
	}
	return types.ContainerJSON{}, nil
}

func (f *fakeExecutionRepo) PullImage(ctx context.Context, ref string) (io.ReadCloser, error) {
	if f.PullFunc != nil {
		return f.PullFunc(ctx, ref)
	}
	return io.NopCloser(strings.NewReader("")), nil
}

func (f *fakeExecutionRepo) StopContainer(ctx context.Context, id string, timeout *int) error {
	if f.StopFunc != nil {
		return f.StopFunc(ctx, id, timeout)
	}
	return nil
}

func (f *fakeExecutionRepo) RemoveContainer(ctx context.Context, id string) error {
	if f.RemoveFunc != nil {
		return f.RemoveFunc(ctx, id)
	}
	return nil
}

func (f *fakeExecutionRepo) CreateContainer(ctx context.Context, name string, cfg *container.Config, hostCfg *container.HostConfig, netCfg *network.NetworkingConfig) (string, error) {
	f.CreatedName = name
	f.CreatedCfg = cfg
	if f.CreateFunc != nil {
		return f.CreateFunc(ctx, name, cfg, hostCfg, netCfg)
	}
	return "new-container-id", nil
}

func (f *fakeExecutionRepo) StartContainer(ctx context.Context, id string) error {
	if f.StartFunc != nil {
		return f.StartFunc(ctx, id)
	}
	return nil
}

func TestExecuteContainerRollbackStandard(t *testing.T) {
	// Temp directory for baseDir/rollbacks
	tempDir, err := os.MkdirTemp("", "execution-test")
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

	manifest := model.RollbackManifest{
		TargetType: model.RollbackTargetContainer,
		TargetName: "api",
		CreatedAt:  time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC),
		Items: []model.RollbackItem{
			{
				Name:          "api",
				RollbackImage: "example/api:old",
				Config: model.ContainerConfig{
					Name:  "api",
					Image: "example/api:new",
					Env:   []string{"ENV_VAR=manifest-val"},
				},
			},
		},
	}

	snapshotDir, err := WriteRollbackSnapshot("rollbacks", manifest)
	if err != nil {
		t.Fatalf("WriteRollbackSnapshot error = %v", err)
	}

	fakeRepo := &fakeExecutionRepo{
		InspectFunc: func(ctx context.Context, id string) (types.ContainerJSON, error) {
			return types.ContainerJSON{
				ContainerJSONBase: &types.ContainerJSONBase{
					Name: "/api",
				},
				Config: &container.Config{
					Image: "example/api:current",
					Env:   []string{"ENV_VAR=current-val"},
				},
			}, nil
		},
	}

	cs := NewContainerService(fakeRepo, nil)
	ch := make(chan PullEvent, 100)

	err = cs.ExecuteContainerRollback(context.Background(), "api", filepath.Join(filepath.Base(snapshotDir), "docker-compose.rollback.yaml"), model.RollbackRestoreStandard, true, ch)
	if err != nil {
		t.Fatalf("ExecuteContainerRollback() error = %v", err)
	}

	if fakeRepo.CreatedCfg == nil {
		t.Fatalf("expected container to be created")
	}

	if fakeRepo.CreatedCfg.Image != "example/api:old" {
		t.Errorf("image = %q, want %q", fakeRepo.CreatedCfg.Image, "example/api:old")
	}

	// In Standard mode, current config should be used: Env should contain current-val, not manifest-val
	if len(fakeRepo.CreatedCfg.Env) != 1 || fakeRepo.CreatedCfg.Env[0] != "ENV_VAR=current-val" {
		t.Errorf("env = %v, want ENV_VAR=current-val", fakeRepo.CreatedCfg.Env)
	}
}

func TestExecuteContainerRollbackAdvanced(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "execution-test-advanced")
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

	manifest := model.RollbackManifest{
		TargetType: model.RollbackTargetContainer,
		TargetName: "api",
		CreatedAt:  time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC),
		Items: []model.RollbackItem{
			{
				Name:          "api",
				RollbackImage: "example/api:old",
				Config: model.ContainerConfig{
					Name:  "api",
					Image: "example/api:new",
					Env:   []string{"ENV_VAR=manifest-val"},
				},
			},
		},
	}

	snapshotDir, err := WriteRollbackSnapshot("rollbacks", manifest)
	if err != nil {
		t.Fatalf("WriteRollbackSnapshot error = %v", err)
	}

	fakeRepo := &fakeExecutionRepo{}

	cs := NewContainerService(fakeRepo, nil)
	ch := make(chan PullEvent, 100)

	err = cs.ExecuteContainerRollback(context.Background(), "api", filepath.Join(filepath.Base(snapshotDir), "docker-compose.rollback.yaml"), model.RollbackRestoreAdvanced, true, ch)
	if err != nil {
		t.Fatalf("ExecuteContainerRollback() error = %v", err)
	}

	if fakeRepo.CreatedCfg == nil {
		t.Fatalf("expected container to be created")
	}

	if fakeRepo.CreatedCfg.Image != "example/api:old" {
		t.Errorf("image = %q, want %q", fakeRepo.CreatedCfg.Image, "example/api:old")
	}

	// In Advanced mode, snapshot config should be used: Env should contain manifest-val
	if len(fakeRepo.CreatedCfg.Env) != 1 || fakeRepo.CreatedCfg.Env[0] != "ENV_VAR=manifest-val" {
		t.Errorf("env = %v, want ENV_VAR=manifest-val", fakeRepo.CreatedCfg.Env)
	}
}

func TestExecuteContainerRollbackTargetMismatch(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "execution-test-mismatch")
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

	manifest := model.RollbackManifest{
		TargetType: model.RollbackTargetContainer,
		TargetName: "api-real",
		Items: []model.RollbackItem{
			{Name: "api-real"},
		},
	}

	snapshotDir, _ := WriteRollbackSnapshot("rollbacks", manifest)
	fakeRepo := &fakeExecutionRepo{}
	cs := NewContainerService(fakeRepo, nil)
	ch := make(chan PullEvent, 100)

	// Invoke with name "api-mismatch" but passing a path that resolves to the "api-real" snapshot
	err = cs.ExecuteContainerRollback(context.Background(), "api-mismatch", filepath.Join("..", "api-real", filepath.Base(snapshotDir), "docker-compose.rollback.yaml"), model.RollbackRestoreStandard, true, ch)
	if err == nil {
		t.Fatalf("expected error due to target mismatch, but got nil")
	}

	if !strings.Contains(err.Error(), "target mismatch") {
		t.Errorf("unexpected error: %v", err)
	}
}

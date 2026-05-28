package service

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"backend/internal/model"
	"backend/internal/repository"
)

func WriteStackRollback(
	ctx context.Context,
	stack model.ComposeStack,
	repo repository.ContainerRepository,
	includeEnv bool,
	mode string,
) (string, error) {
	configs := make([]model.ContainerConfig, 0, len(stack.Services))
	rollbackImages := make(map[string]string, len(stack.Services))

	for _, svc := range stack.Services {
		rollbackImages[svc.Name] = pinImageDigest(svc.Image, svc.LocalDigest)

		if svc.ContainerID != "" {
			inspect, err := repo.InspectContainer(ctx, svc.ContainerID)
			if err != nil {
				return "", fmt.Errorf("inspect %s: %w", svc.Name, err)
			}
			cfg := captureContainerConfig(inspect)
			configs = append(configs, cfg)
		}
	}

	now := time.Now().UTC()
	manifest := BuildStackRollbackManifest(stack, configs, rollbackImages, model.RollbackSourceMode(mode), now)
	snapshotDir, err := WriteRollbackSnapshot("rollbacks", manifest)
	if err != nil {
		return "", err
	}
	return snapshotDir, nil
}

func pinImageDigest(image, localDigest string) string {
	if localDigest == "" {
		return image
	}
	ref := image
	if idx := strings.Index(ref, "@"); idx != -1 {
		ref = ref[:idx]
	}
	if idx := strings.LastIndex(ref, ":"); idx != -1 {
		if !strings.Contains(ref[idx+1:], "/") {
			ref = ref[:idx]
		}
	}
	return ref + "@" + localDigest
}

func configFileAccessible(configFiles []string) bool {
	if len(configFiles) == 0 {
		return false
	}
	_, err := os.Stat(configFiles[0])
	return err == nil
}

func ListStackRollbacks(stackName string) ([]model.RollbackFile, error) {
	return ListRollbackSnapshots("rollbacks", stackName)
}

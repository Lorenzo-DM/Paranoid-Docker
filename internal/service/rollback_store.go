package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"backend/internal/model"
)

const (
	rollbackManifestFilename = "rollback-manifest.json"
	rollbackComposeFilename  = "docker-compose.rollback.yaml"
)

func WriteRollbackSnapshot(baseDir string, manifest model.RollbackManifest) (string, error) {
	if manifest.TargetName == "" {
		return "", fmt.Errorf("rollback manifest missing target name")
	}
	if manifest.CreatedAt.IsZero() {
		manifest.CreatedAt = time.Now().UTC()
	} else {
		manifest.CreatedAt = manifest.CreatedAt.UTC()
	}

	timestamp := manifest.CreatedAt.Format("2006-01-02T15-04-05")
	snapshotDir := filepath.Join(baseDir, manifest.TargetName, timestamp)
	if err := os.MkdirAll(snapshotDir, 0o755); err != nil {
		return "", fmt.Errorf("create rollback snapshot dir: %w", err)
	}

	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal rollback manifest: %w", err)
	}
	manifestData = append(manifestData, '\n')
	if err := os.WriteFile(filepath.Join(snapshotDir, rollbackManifestFilename), manifestData, 0o644); err != nil {
		return "", fmt.Errorf("write rollback manifest: %w", err)
	}

	composeData, err := RenderRollbackCompose(manifest, false)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(snapshotDir, rollbackComposeFilename), composeData, 0o644); err != nil {
		return "", fmt.Errorf("write rollback compose: %w", err)
	}

	return snapshotDir, nil
}

func ReadRollbackManifest(path string) (model.RollbackManifest, error) {
	info, err := os.Stat(path)
	if err != nil {
		return model.RollbackManifest{}, err
	}
	if info.IsDir() {
		path = filepath.Join(path, rollbackManifestFilename)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return model.RollbackManifest{}, err
	}

	var manifest model.RollbackManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return model.RollbackManifest{}, fmt.Errorf("decode rollback manifest: %w", err)
	}
	return manifest, nil
}

func ListRollbackSnapshots(baseDir string, targetName string) ([]model.RollbackFile, error) {
	targetDir := filepath.Join(baseDir, targetName)
	entries, err := os.ReadDir(targetDir)
	if os.IsNotExist(err) {
		return []model.RollbackFile{}, nil
	}
	if err != nil {
		return nil, err
	}

	files := make([]model.RollbackFile, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		timestamp := entry.Name()
		snapshotDir := filepath.Join(targetDir, timestamp)
		composePath := filepath.Join(snapshotDir, rollbackComposeFilename)
		info, err := os.Stat(composePath)
		if err != nil {
			continue
		}

		createdAt := info.ModTime()
		previousImage := parsePreviousImageFromFile(composePath)
		if manifest, err := ReadRollbackManifest(snapshotDir); err == nil {
			if !manifest.CreatedAt.IsZero() {
				createdAt = manifest.CreatedAt
			}
			if images := rollbackImages(manifest); len(images) > 0 {
				previousImage = strings.Join(images, ", ")
			}
		}

		files = append(files, model.RollbackFile{
			Filename:      filepath.Join(timestamp, rollbackComposeFilename),
			Path:          composePath,
			CreatedAt:     createdAt,
			PreviousImage: previousImage,
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].CreatedAt.After(files[j].CreatedAt)
	})
	return files, nil
}

func rollbackImages(manifest model.RollbackManifest) []string {
	images := make([]string, 0, len(manifest.Items))
	seen := map[string]struct{}{}
	for _, item := range manifest.Items {
		image := item.RollbackImage
		if image == "" {
			image = item.Config.Image
		}
		if image == "" {
			continue
		}
		if _, ok := seen[image]; ok {
			continue
		}
		seen[image] = struct{}{}
		images = append(images, image)
	}
	return images
}

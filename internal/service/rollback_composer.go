package service

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"backend/internal/model"
)

type composeFile struct {
	Services map[string]composeService `yaml:"services"`
	Networks map[string]composeNetwork `yaml:"networks,omitempty"`
	Volumes  map[string]composeVolume  `yaml:"volumes,omitempty"`
}

type composeService struct {
	Image         string            `yaml:"image"`
	ContainerName string            `yaml:"container_name"`
	Restart       string            `yaml:"restart,omitempty"`
	Ports         []string          `yaml:"ports,omitempty"`
	Volumes       []string          `yaml:"volumes,omitempty"`
	Environment   []string          `yaml:"environment,omitempty"`
	Networks      []string          `yaml:"networks,omitempty"`
	Labels        map[string]string `yaml:"labels,omitempty"`
}

type composeNetwork struct {
	External bool `yaml:"external"`
}

type composeVolume struct {
	External bool `yaml:"external"`
}

func WriteRollbackCompose(cfg model.ContainerConfig, imageDigest string, includeEnv bool) (string, error) {
	now := time.Now().UTC()
	manifest := BuildContainerRollbackManifest(cfg, imageDigest, now)
	snapshotDir, err := WriteRollbackSnapshot("rollbacks", manifest)
	if err != nil {
		return "", err
	}
	return filepath.Join(snapshotDir, rollbackComposeFilename), nil
}

func restartPolicyName(name string) string {
	switch name {
	case "always", "unless-stopped", "on-failure":
		return name
	default:
		return ""
	}
}

func ListRollbacks(containerName string) ([]model.RollbackFile, error) {
	return ListRollbackSnapshots("rollbacks", containerName)
}

func parsePreviousImageFromFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if after, ok := strings.CutPrefix(line, "# Previous image: "); ok {
			return after
		}
	}
	return ""
}

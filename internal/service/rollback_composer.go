package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"backend/internal/model"

	"gopkg.in/yaml.v3"
)

// RollbackWriter writes and lists rollback compose files under BaseDir.
type RollbackWriter struct {
	BaseDir string
	Runner  CommandRunner
}

func NewRollbackWriter(baseDir string, runner CommandRunner) *RollbackWriter {
	return &RollbackWriter{BaseDir: baseDir, Runner: runner}
}

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

func (w *RollbackWriter) WriteRollbackCompose(cfg model.ContainerConfig, imageDigest string, includeEnv bool) (string, error) {
	now := time.Now().UTC()
	timestamp := now.Format("2006-01-02T15-04-05")

	dir := filepath.Join(w.BaseDir, cfg.Name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create rollbacks dir: %w", err)
	}

	filename := fmt.Sprintf("%s_rollback.yaml", timestamp)
	path := filepath.Join(dir, filename)

	pinnedImage := imageDigest
	if pinnedImage == "" {
		pinnedImage = cfg.Image
	}

	var env []string
	if includeEnv {
		env = cfg.Env
	}

	svc := composeService{
		Image:         pinnedImage,
		ContainerName: cfg.Name,
		Restart:       restartPolicyName(string(cfg.RestartPolicy.Name)),
		Environment:   env,
		Networks:      cfg.Networks,
	}

	for port, bindings := range cfg.PortBindings {
		for _, b := range bindings {
			hostPort := b.HostPort
			cp := port.Port()
			proto := port.Proto()
			if hostPort != "" {
				svc.Ports = append(svc.Ports, fmt.Sprintf("%s:%s/%s", hostPort, cp, proto))
			}
		}
	}

	svc.Volumes = append(svc.Volumes, cfg.Binds...)

	networks := map[string]composeNetwork{}
	volumes := map[string]composeVolume{}

	for _, m := range cfg.Mounts {
		if m.Type == "volume" && m.Name != "" {
			svc.Volumes = append(svc.Volumes, fmt.Sprintf("%s:%s", m.Name, m.Destination))
			volumes[m.Name] = composeVolume{External: true}
		}
	}

	labels := map[string]string{}
	for k, v := range cfg.Labels {
		if !strings.HasPrefix(k, "com.docker.compose.") {
			labels[k] = v
		}
	}
	if len(labels) > 0 {
		svc.Labels = labels
	}

	for _, n := range cfg.Networks {
		networks[n] = composeNetwork{External: true}
	}

	cf := composeFile{
		Services: map[string]composeService{cfg.Name: svc},
	}
	if len(networks) > 0 {
		cf.Networks = networks
	}
	if len(volumes) > 0 {
		cf.Volumes = volumes
	}

	header := fmt.Sprintf("# Rollback for container: %s\n# Generated: %s\n# Previous image: %s\n\n",
		cfg.Name, now.Format(time.RFC3339), pinnedImage)

	data, err := yaml.Marshal(cf)
	if err != nil {
		return "", fmt.Errorf("marshal compose: %w", err)
	}

	if err := os.WriteFile(path, append([]byte(header), data...), 0o644); err != nil {
		return "", fmt.Errorf("write rollback file: %w", err)
	}

	return path, nil
}

func restartPolicyName(name string) string {
	switch name {
	case "always", "unless-stopped", "on-failure":
		return name
	default:
		return ""
	}
}

func (w *RollbackWriter) ListRollbacks(containerName string) ([]model.RollbackFile, error) {
	dir := filepath.Join(w.BaseDir, containerName)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []model.RollbackFile{}, nil
	}
	if err != nil {
		return nil, err
	}

	var files []model.RollbackFile
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		path := filepath.Join(dir, e.Name())
		prev := parsePreviousImageFromFile(path)

		files = append(files, model.RollbackFile{
			Filename:      e.Name(),
			Path:          path,
			CreatedAt:     info.ModTime(),
			PreviousImage: prev,
		})
	}
	return files, nil
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

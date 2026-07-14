package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"backend/internal/model"
	"backend/internal/repository"

	"gopkg.in/yaml.v3"
)

func WriteStackRollback(
	ctx context.Context,
	stack model.ComposeStack,
	repo repository.ContainerRepository,
	includeEnv bool,
	mode string,
	runner CommandRunner,
) (string, error) {
	now := time.Now().UTC()
	ts := now.Format("2006-01-02T15-04-05")
	dir := filepath.Join("rollbacks", stack.Name, ts)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir rollback dir: %w", err)
	}

	if mode == "compose" {
		return writeRollbackFromComposeConfig(ctx, stack, dir, now, runner)
	}
	return writeRollbackFromInspect(ctx, stack, repo, includeEnv, dir, now)
}

func configFileAccessible(configFiles []string) bool {
	if len(configFiles) == 0 {
		return false
	}
	_, err := os.Stat(configFiles[0])
	return err == nil
}

func writeRollbackFromComposeConfig(
	ctx context.Context,
	stack model.ComposeStack,
	dir string,
	now time.Time,
	runner CommandRunner,
) (string, error) {
	args := []string{"compose"}
	for _, f := range stack.ConfigFiles {
		args = append(args, "-f", f)
	}
	args = append(args, "config")

	data, err := runner.Output(ctx, stack.WorkingDir, "docker", args...)
	if err != nil {
		return dir, fmt.Errorf("docker compose config: %w", err)
	}

	pinned := buildPinnedImages(stack)
	patched, err := patchComposeImages(data, pinned)
	if err != nil {
		patched = data
	}

	header := fmt.Sprintf(
		"# Rollback for stack: %s\n# Generated: %s\n# Source: docker compose config\n\n",
		stack.Name, now.Format(time.RFC3339),
	)

	destPath := filepath.Join(dir, "docker-compose.yaml")
	if err := os.WriteFile(destPath, append([]byte(header), patched...), 0o644); err != nil {
		return dir, fmt.Errorf("write rollback file: %w", err)
	}
	return dir, nil
}

func writeRollbackFromInspect(
	ctx context.Context,
	stack model.ComposeStack,
	repo repository.ContainerRepository,
	includeEnv bool,
	dir string,
	now time.Time,
) (string, error) {
	cf := composeFile{
		Services: make(map[string]composeService, len(stack.Services)),
	}
	allNetworks := map[string]composeNetwork{}
	allVolumes := map[string]composeVolume{}

	for _, svc := range stack.Services {
		if svc.ContainerID == "" {
			continue
		}

		inspect, err := repo.InspectContainer(ctx, svc.ContainerID)
		if err != nil {
			return dir, fmt.Errorf("inspect %s: %w", svc.Name, err)
		}

		cfg := captureContainerConfig(inspect)
		pinnedImage := pinImageDigest(svc.Image, svc.LocalDigest)

		var env []string
		if includeEnv {
			env = cfg.Env
		}

		cs := composeService{
			Image:         pinnedImage,
			ContainerName: cfg.Name,
			Restart:       restartPolicyName(string(cfg.RestartPolicy.Name)),
			Environment:   env,
			Networks:      cfg.Networks,
		}

		for port, bindings := range cfg.PortBindings {
			for _, b := range bindings {
				if b.HostPort != "" {
					cs.Ports = append(cs.Ports, fmt.Sprintf("%s:%s/%s", b.HostPort, port.Port(), port.Proto()))
				}
			}
		}

		cs.Volumes = append(cs.Volumes, cfg.Binds...)

		for _, m := range cfg.Mounts {
			if m.Type == "volume" && m.Name != "" {
				cs.Volumes = append(cs.Volumes, fmt.Sprintf("%s:%s", m.Name, m.Destination))
				allVolumes[m.Name] = composeVolume{External: true}
			}
		}

		labels := map[string]string{}
		for k, v := range cfg.Labels {
			if !strings.HasPrefix(k, "com.docker.compose.") {
				labels[k] = v
			}
		}
		if len(labels) > 0 {
			cs.Labels = labels
		}

		for _, n := range cfg.Networks {
			allNetworks[n] = composeNetwork{External: true}
		}

		cf.Services[svc.Name] = cs
	}

	if len(allNetworks) > 0 {
		cf.Networks = allNetworks
	}
	if len(allVolumes) > 0 {
		cf.Volumes = allVolumes
	}

	data, err := yaml.Marshal(cf)
	if err != nil {
		return dir, fmt.Errorf("marshal compose: %w", err)
	}

	var imageList []string
	for name, s := range cf.Services {
		imageList = append(imageList, fmt.Sprintf("#   %s: %s", name, s.Image))
	}

	header := fmt.Sprintf(
		"# Rollback for stack: %s\n# Generated: %s\n# Source: docker inspect\n# Services:\n%s\n\n",
		stack.Name, now.Format(time.RFC3339), strings.Join(imageList, "\n"),
	)

	destPath := filepath.Join(dir, "docker-compose.yaml")
	if err := os.WriteFile(destPath, append([]byte(header), data...), 0o644); err != nil {
		return dir, fmt.Errorf("write rollback file: %w", err)
	}
	return dir, nil
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

func buildPinnedImages(stack model.ComposeStack) map[string]string {
	result := make(map[string]string, len(stack.Services))
	for _, svc := range stack.Services {
		if svc.ContainerID == "" || svc.LocalDigest == "" {
			continue
		}
		result[svc.Name] = pinImageDigest(svc.Image, svc.LocalDigest)
	}
	return result
}

func patchComposeImages(data []byte, pinned map[string]string) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return data, nil
	}

	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return data, nil
	}

	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value != "services" {
			continue
		}
		svcs := root.Content[i+1]
		if svcs.Kind != yaml.MappingNode {
			continue
		}
		for j := 0; j+1 < len(svcs.Content); j += 2 {
			svcName := svcs.Content[j].Value
			svcNode := svcs.Content[j+1]
			if pinnedRef, ok := pinned[svcName]; ok {
				replaceImageInService(svcNode, pinnedRef)
			}
		}
	}

	return yaml.Marshal(&doc)
}

func replaceImageInService(svcNode *yaml.Node, pinnedRef string) {
	if svcNode.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(svcNode.Content); i += 2 {
		if svcNode.Content[i].Value == "image" {
			svcNode.Content[i+1].Value = pinnedRef
			return
		}
	}
}

func ListStackRollbacks(stackName string) ([]model.RollbackFile, error) {
	base := filepath.Join("rollbacks", stackName)
	entries, err := os.ReadDir(base)
	if os.IsNotExist(err) {
		return []model.RollbackFile{}, nil
	}
	if err != nil {
		return nil, err
	}

	var files []model.RollbackFile
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		subDir := filepath.Join(base, e.Name())
		subEntries, _ := os.ReadDir(subDir)
		for _, f := range subEntries {
			if !strings.HasSuffix(f.Name(), ".yaml") && !strings.HasSuffix(f.Name(), ".yml") {
				continue
			}
			info, _ := f.Info()
			path := filepath.Join(subDir, f.Name())
			files = append(files, model.RollbackFile{
				Filename:      filepath.Join(e.Name(), f.Name()),
				Path:          path,
				CreatedAt:     info.ModTime(),
				PreviousImage: parsePreviousImageFromFile(path),
			})
		}
	}
	return files, nil
}

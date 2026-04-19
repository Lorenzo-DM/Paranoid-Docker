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
) (string, error) {
	now := time.Now().UTC()
	ts := now.Format("2006-01-02T15-04-05")
	dir := filepath.Join("rollbacks", stack.Name, ts)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir rollback dir: %w", err)
	}

	pinned := buildPinnedImages(stack)

	var envMap map[string][]string
	if includeEnv {
		envMap = buildServiceEnvMap(ctx, stack, repo)
	}

	for _, configPath := range stack.ConfigFiles {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return dir, fmt.Errorf("read %s: %w", configPath, err)
		}

		patched, err := patchComposeFile(data, pinned, envMap)
		if err != nil {
			patched = data
		}

		header := fmt.Sprintf(
			"# Rollback for stack: %s\n# Generated: %s\n# Original: %s\n\n",
			stack.Name, now.Format(time.RFC3339), configPath,
		)

		destName := filepath.Base(configPath)
		destPath := filepath.Join(dir, destName)
		if err := os.WriteFile(destPath, append([]byte(header), patched...), 0o644); err != nil {
			return dir, fmt.Errorf("write rollback file: %w", err)
		}
	}

	return dir, nil
}

func buildServiceEnvMap(ctx context.Context, stack model.ComposeStack, repo repository.ContainerRepository) map[string][]string {
	result := make(map[string][]string)
	for _, svc := range stack.Services {
		if svc.ContainerID == "" {
			continue
		}
		inspect, err := repo.InspectContainer(ctx, svc.ContainerID)
		if err == nil && inspect.Config != nil {
			result[svc.Name] = inspect.Config.Env
		}
	}
	return result
}

func buildPinnedImages(stack model.ComposeStack) map[string]string {
	result := make(map[string]string)
	for _, svc := range stack.Services {
		if svc.ContainerID == "" || svc.LocalDigest == "" {
			continue
		}
		imageRef := svc.Image
		if idx := strings.Index(imageRef, "@"); idx != -1 {
			imageRef = imageRef[:idx]
		}
		if idx := strings.LastIndex(imageRef, ":"); idx != -1 {
			if !strings.Contains(imageRef[idx+1:], "/") {
				imageRef = imageRef[:idx]
			}
		}
		result[svc.Name] = imageRef + "@" + svc.LocalDigest
	}
	return result
}

func patchComposeFile(data []byte, pinned map[string]string, envMap map[string][]string) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return data, nil
	}

	root := doc.Content[0]
	patchYAMLServices(root, pinned, envMap)

	return yaml.Marshal(&doc)
}

func patchYAMLServices(node *yaml.Node, pinned map[string]string, envMap map[string][]string) {
	if node.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i]
		val := node.Content[i+1]
		if key.Value == "services" && val.Kind == yaml.MappingNode {
			for j := 0; j+1 < len(val.Content); j += 2 {
				svcName := val.Content[j].Value
				svcNode := val.Content[j+1]
				if pinnedRef, ok := pinned[svcName]; ok {
					replaceImageInService(svcNode, pinnedRef)
				}
				if envMap != nil {
					if env, ok := envMap[svcName]; ok && len(env) > 0 {
						setEnvInService(svcNode, env)
					}
				}
			}
		}
	}
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

func setEnvInService(svcNode *yaml.Node, env []string) {
	if svcNode.Kind != yaml.MappingNode {
		return
	}

	seqNode := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, e := range env {
		seqNode.Content = append(seqNode.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: e})
	}

	for i := 0; i+1 < len(svcNode.Content); i += 2 {
		if svcNode.Content[i].Value == "environment" {
			svcNode.Content[i+1] = seqNode
			return
		}
	}

	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "environment"}
	svcNode.Content = append(svcNode.Content, keyNode, seqNode)
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

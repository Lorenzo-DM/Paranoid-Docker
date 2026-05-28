package service

import (
	"strings"
	"time"

	"backend/internal/model"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/go-connections/nat"
)

const maskedEnvValue = "********"

func MaskEnv(env []string) map[string]string {
	masked := make(map[string]string, len(env))
	for _, entry := range env {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			key = entry
		}
		masked[key] = maskedEnvValue
	}
	return masked
}

func BuildContainerRollbackManifest(cfg model.ContainerConfig, rollbackImage string, now time.Time) model.RollbackManifest {
	if rollbackImage == "" {
		rollbackImage = cfg.Image
	}
	configSnapshot := cloneContainerConfig(cfg)

	return model.RollbackManifest{
		Version:    1,
		TargetType: model.RollbackTargetContainer,
		TargetName: cfg.Name,
		CreatedAt:  now,
		SourceMode: model.RollbackSourceInspect,
		Items: []model.RollbackItem{
			{
				Name:          cfg.Name,
				CurrentImage:  cfg.Image,
				RollbackImage: rollbackImage,
				// Raw env is intentionally retained for advanced restore; UI previews must use EnvMasked.
				Config:    configSnapshot,
				EnvMasked: MaskEnv(configSnapshot.Env),
			},
		},
	}
}

func BuildStackRollbackManifest(
	stack model.ComposeStack,
	configs []model.ContainerConfig,
	rollbackImages map[string]string,
	mode model.RollbackSourceMode,
	now time.Time,
) model.RollbackManifest {
	if mode == "" || len(stack.ConfigFiles) == 0 {
		mode = model.RollbackSourceInspect
	}

	items := make([]model.RollbackItem, 0, len(configs))
	for _, cfg := range configs {
		service := matchComposeService(stack.Services, cfg)
		serviceName := service.Name
		configSnapshot := cloneContainerConfig(cfg)

		rollbackImage := ""
		if serviceName != "" {
			rollbackImage = rollbackImages[serviceName]
		}
		if rollbackImage == "" {
			rollbackImage = rollbackImages[cfg.Name]
		}
		if rollbackImage == "" {
			rollbackImage = cfg.Image
		}

		items = append(items, model.RollbackItem{
			Name:          cfg.Name,
			ContainerID:   service.ContainerID,
			ServiceName:   serviceName,
			CurrentImage:  cfg.Image,
			RollbackImage: rollbackImage,
			// Raw env is intentionally retained for advanced restore; UI previews must use EnvMasked.
			Config:    configSnapshot,
			EnvMasked: MaskEnv(configSnapshot.Env),
		})
	}

	return model.RollbackManifest{
		Version:      1,
		TargetType:   model.RollbackTargetStack,
		TargetName:   stack.Name,
		CreatedAt:    now,
		SourceMode:   mode,
		ComposeFiles: append([]string(nil), stack.ConfigFiles...),
		WorkingDir:   stack.WorkingDir,
		Items:        items,
	}
}

func matchComposeService(services []model.ComposeService, cfg model.ContainerConfig) model.ComposeService {
	if cfg.Labels != nil {
		if serviceName := cfg.Labels["com.docker.compose.service"]; serviceName != "" {
			for _, service := range services {
				if service.Name == serviceName {
					return service
				}
			}
			return model.ComposeService{Name: serviceName}
		}
	}

	for _, service := range services {
		if service.Name == cfg.Name {
			return service
		}
	}

	return model.ComposeService{Name: cfg.Name}
}

func cloneContainerConfig(cfg model.ContainerConfig) model.ContainerConfig {
	clone := cfg
	clone.Cmd = append([]string(nil), cfg.Cmd...)
	clone.Entrypoint = append([]string(nil), cfg.Entrypoint...)
	clone.Env = append([]string(nil), cfg.Env...)
	clone.Binds = append([]string(nil), cfg.Binds...)
	clone.Mounts = append([]dockertypes.MountPoint(nil), cfg.Mounts...)
	clone.Networks = append([]string(nil), cfg.Networks...)

	if cfg.Labels != nil {
		clone.Labels = make(map[string]string, len(cfg.Labels))
		for key, value := range cfg.Labels {
			clone.Labels[key] = value
		}
	}

	if cfg.PortBindings != nil {
		clone.PortBindings = make(nat.PortMap, len(cfg.PortBindings))
		for port, bindings := range cfg.PortBindings {
			clone.PortBindings[port] = append([]nat.PortBinding(nil), bindings...)
		}
	}

	return clone
}

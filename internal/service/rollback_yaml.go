package service

import (
	"fmt"
	"strings"

	"backend/internal/model"

	"gopkg.in/yaml.v3"
)

func RenderRollbackCompose(manifest model.RollbackManifest, includeEnv bool) ([]byte, error) {
	cf := composeFile{
		Services: make(map[string]composeService, len(manifest.Items)),
	}
	networks := map[string]composeNetwork{}
	volumes := map[string]composeVolume{}

	for _, item := range manifest.Items {
		cfg := item.Config
		serviceName := item.ServiceName
		if serviceName == "" {
			serviceName = item.Name
		}
		if serviceName == "" {
			serviceName = cfg.Name
		}
		if serviceName == "" {
			return nil, fmt.Errorf("rollback item missing service name")
		}

		containerName := cfg.Name
		if containerName == "" {
			containerName = item.Name
		}

		image := item.RollbackImage
		if image == "" {
			image = cfg.Image
		}

		svc := composeService{
			Image:         image,
			ContainerName: containerName,
			Restart:       restartPolicyName(string(cfg.RestartPolicy.Name)),
			Networks:      append([]string(nil), cfg.Networks...),
		}
		if includeEnv {
			svc.Environment = append([]string(nil), cfg.Env...)
		}

		for port, bindings := range cfg.PortBindings {
			for _, binding := range bindings {
				if binding.HostPort == "" {
					continue
				}
				host := binding.HostPort
				if binding.HostIP != "" {
					host = binding.HostIP + ":" + host
				}
				svc.Ports = append(svc.Ports, fmt.Sprintf("%s:%s/%s", host, port.Port(), port.Proto()))
			}
		}

		svc.Volumes = append(svc.Volumes, cfg.Binds...)
		for _, mount := range cfg.Mounts {
			if mount.Type == "volume" && mount.Name != "" {
				svc.Volumes = append(svc.Volumes, fmt.Sprintf("%s:%s", mount.Name, mount.Destination))
				volumes[mount.Name] = composeVolume{External: true}
			}
		}

		labels := map[string]string{}
		for key, value := range cfg.Labels {
			if !strings.HasPrefix(key, "com.docker.compose.") {
				labels[key] = value
			}
		}
		if len(labels) > 0 {
			svc.Labels = labels
		}

		for _, network := range cfg.Networks {
			if network != "" {
				networks[network] = composeNetwork{External: true}
			}
		}

		cf.Services[serviceName] = svc
	}

	if len(networks) > 0 {
		cf.Networks = networks
	}
	if len(volumes) > 0 {
		cf.Volumes = volumes
	}

	data, err := yaml.Marshal(cf)
	if err != nil {
		return nil, fmt.Errorf("marshal rollback compose: %w", err)
	}
	return data, nil
}

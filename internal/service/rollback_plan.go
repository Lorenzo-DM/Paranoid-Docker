package service

import (
	"fmt"

	"backend/internal/model"
)

func BuildRollbackPlan(manifest model.RollbackManifest, mode model.RollbackRestoreMode) ([]model.RollbackAction, []string) {
	var actions []model.RollbackAction
	var warnings []string

	if manifest.TargetType == model.RollbackTargetContainer {
		if len(manifest.Items) == 0 {
			warnings = append(warnings, "No items found in rollback manifest")
			return actions, warnings
		}
		item := manifest.Items[0]

		// Pull action
		actions = append(actions, model.RollbackAction{
			Type:        "pull",
			Target:      item.RollbackImage,
			Description: fmt.Sprintf("Pull rollback image: %s", item.RollbackImage),
			Destructive: false,
		})

		isDestructive := mode == model.RollbackRestoreAdvanced

		// Stop
		actions = append(actions, model.RollbackAction{
			Type:        "stop",
			Target:      item.Name,
			Description: fmt.Sprintf("Stop container: %s", item.Name),
			Destructive: isDestructive,
		})

		// Remove
		actions = append(actions, model.RollbackAction{
			Type:        "remove",
			Target:      item.Name,
			Description: fmt.Sprintf("Remove container: %s", item.Name),
			Destructive: isDestructive,
		})

		// Create
		createDesc := fmt.Sprintf("Create container: %s using current configuration with rollback image", item.Name)
		if mode == model.RollbackRestoreAdvanced {
			createDesc = fmt.Sprintf("Create container: %s using configuration from snapshot with rollback image", item.Name)
		}
		actions = append(actions, model.RollbackAction{
			Type:        "create",
			Target:      item.Name,
			Description: createDesc,
			Destructive: isDestructive,
		})

		// Start
		actions = append(actions, model.RollbackAction{
			Type:        "start",
			Target:      item.Name,
			Description: fmt.Sprintf("Start container: %s", item.Name),
			Destructive: false,
		})

		// Network connections (if any)
		for _, net := range item.Config.Networks {
			if net != "" && net != "default" {
				actions = append(actions, model.RollbackAction{
					Type:        "connect_network",
					Target:      fmt.Sprintf("%s -> %s", item.Name, net),
					Description: fmt.Sprintf("Connect container %s to network %s", item.Name, net),
					Destructive: false,
				})
			}
		}

		if item.Config.Labels != nil {
			if _, isCompose := item.Config.Labels["com.docker.compose.project"]; isCompose {
				warnings = append(warnings, "Container is managed by Docker Compose. Recreating it standalone might conflict with the Compose project.")
			}
		}
	} else if manifest.TargetType == model.RollbackTargetStack {
		if manifest.SourceMode == model.RollbackSourceCompose {
			for _, item := range manifest.Items {
				actions = append(actions, model.RollbackAction{
					Type:        "pull",
					Target:      item.RollbackImage,
					Description: fmt.Sprintf("Pull rollback image for service %s: %s", item.ServiceName, item.RollbackImage),
					Destructive: false,
				})
			}

			isDestructive := mode == model.RollbackRestoreAdvanced
			actions = append(actions, model.RollbackAction{
				Type:        "compose_up",
				Target:      manifest.TargetName,
				Description: fmt.Sprintf("Deploy stack %s with rollback image overrides using docker compose up -d", manifest.TargetName),
				Destructive: isDestructive,
			})
		} else {
			warnings = append(warnings, "Compose files are not accessible on disk. Rolling back using inspect-based container replication.")

			for _, item := range manifest.Items {
				actions = append(actions, model.RollbackAction{
					Type:        "pull",
					Target:      item.RollbackImage,
					Description: fmt.Sprintf("Pull rollback image for service %s: %s", item.ServiceName, item.RollbackImage),
					Destructive: false,
				})

				isDestructive := mode == model.RollbackRestoreAdvanced

				// Stop
				actions = append(actions, model.RollbackAction{
					Type:        "stop",
					Target:      item.Name,
					Description: fmt.Sprintf("Stop container: %s (service %s)", item.Name, item.ServiceName),
					Destructive: isDestructive,
				})

				// Remove
				actions = append(actions, model.RollbackAction{
					Type:        "remove",
					Target:      item.Name,
					Description: fmt.Sprintf("Remove container: %s (service %s)", item.Name, item.ServiceName),
					Destructive: isDestructive,
				})

				// Create
				createDesc := fmt.Sprintf("Create container: %s using current configuration with rollback image", item.Name)
				if mode == model.RollbackRestoreAdvanced {
					createDesc = fmt.Sprintf("Create container: %s using configuration from snapshot with rollback image", item.Name)
				}
				actions = append(actions, model.RollbackAction{
					Type:        "create",
					Target:      item.Name,
					Description: createDesc,
					Destructive: isDestructive,
				})

				// Start
				actions = append(actions, model.RollbackAction{
					Type:        "start",
					Target:      item.Name,
					Description: fmt.Sprintf("Start container: %s (service %s)", item.Name, item.ServiceName),
					Destructive: false,
				})

				for _, net := range item.Config.Networks {
					if net != "" && net != "default" {
						actions = append(actions, model.RollbackAction{
							Type:        "connect_network",
							Target:      fmt.Sprintf("%s -> %s", item.Name, net),
							Description: fmt.Sprintf("Connect container %s to network %s", item.Name, net),
							Destructive: false,
						})
					}
				}
			}
		}
	}

	return actions, warnings
}

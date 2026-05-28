package service

import (
	"testing"

	"backend/internal/model"
)

func TestBuildRollbackPlanContainerStandard(t *testing.T) {
	manifest := model.RollbackManifest{
		TargetType: model.RollbackTargetContainer,
		Items: []model.RollbackItem{
			{
				Name:          "api",
				RollbackImage: "example/api:old",
				Config: model.ContainerConfig{
					Networks: []string{"default", "custom_net"},
				},
			},
		},
	}

	actions, warnings := BuildRollbackPlan(manifest, model.RollbackRestoreStandard)

	if len(warnings) != 0 {
		t.Fatalf("expected 0 warnings, got %d: %v", len(warnings), warnings)
	}

	// We expect 6 actions: pull, stop, remove, create, start, connect_network
	if len(actions) != 6 {
		t.Fatalf("expected 6 actions, got %d: %#v", len(actions), actions)
	}

	expectedTypes := []string{"pull", "stop", "remove", "create", "start", "connect_network"}
	for i, act := range actions {
		if act.Type != expectedTypes[i] {
			t.Errorf("action %d type = %q, want %q", i, act.Type, expectedTypes[i])
		}
		if act.Destructive {
			t.Errorf("action %d type %q should not be destructive in standard mode", i, act.Type)
		}
	}
}

func TestBuildRollbackPlanContainerAdvanced(t *testing.T) {
	manifest := model.RollbackManifest{
		TargetType: model.RollbackTargetContainer,
		Items: []model.RollbackItem{
			{
				Name:          "api",
				RollbackImage: "example/api:old",
				Config: model.ContainerConfig{
					Networks: []string{"default"},
				},
			},
		},
	}

	actions, warnings := BuildRollbackPlan(manifest, model.RollbackRestoreAdvanced)

	if len(warnings) != 0 {
		t.Fatalf("expected 0 warnings, got %d: %v", len(warnings), warnings)
	}

	// We expect 5 actions: pull, stop, remove, create, start
	if len(actions) != 5 {
		t.Fatalf("expected 5 actions, got %d: %#v", len(actions), actions)
	}

	for _, act := range actions {
		// Stop, remove, and create should be marked destructive in advanced mode
		shouldBeDestructive := act.Type == "stop" || act.Type == "remove" || act.Type == "create"
		if act.Destructive != shouldBeDestructive {
			t.Errorf("action %q destructive = %t, want %t", act.Type, act.Destructive, shouldBeDestructive)
		}
	}
}

func TestBuildRollbackPlanContainerComposeWarning(t *testing.T) {
	manifest := model.RollbackManifest{
		TargetType: model.RollbackTargetContainer,
		Items: []model.RollbackItem{
			{
				Name: "api",
				Config: model.ContainerConfig{
					Labels: map[string]string{
						"com.docker.compose.project": "myproject",
					},
				},
			},
		},
	}

	_, warnings := BuildRollbackPlan(manifest, model.RollbackRestoreStandard)
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}
	expectedWarning := "Container is managed by Docker Compose. Recreating it standalone might conflict with the Compose project."
	if warnings[0] != expectedWarning {
		t.Fatalf("warning = %q, want %q", warnings[0], expectedWarning)
	}
}

func TestBuildRollbackPlanStackComposeMode(t *testing.T) {
	manifest := model.RollbackManifest{
		TargetType: model.RollbackTargetStack,
		TargetName: "my-stack",
		SourceMode: model.RollbackSourceCompose,
		Items: []model.RollbackItem{
			{ServiceName: "web", RollbackImage: "example/web:old"},
			{ServiceName: "db", RollbackImage: "example/db:old"},
		},
	}

	actions, warnings := BuildRollbackPlan(manifest, model.RollbackRestoreStandard)
	if len(warnings) != 0 {
		t.Fatalf("expected 0 warnings, got %d: %v", len(warnings), warnings)
	}

	// Expect 3 actions: pull web, pull db, compose_up
	if len(actions) != 3 {
		t.Fatalf("expected 3 actions, got %d", len(actions))
	}
	if actions[0].Type != "pull" || actions[0].Target != "example/web:old" {
		t.Errorf("action 0 mismatch: %#v", actions[0])
	}
	if actions[1].Type != "pull" || actions[1].Target != "example/db:old" {
		t.Errorf("action 1 mismatch: %#v", actions[1])
	}
	if actions[2].Type != "compose_up" || actions[2].Target != "my-stack" || actions[2].Destructive {
		t.Errorf("action 2 mismatch: %#v", actions[2])
	}
}

func TestBuildRollbackPlanStackInspectMode(t *testing.T) {
	manifest := model.RollbackManifest{
		TargetType: model.RollbackTargetStack,
		TargetName: "my-stack",
		SourceMode: model.RollbackSourceInspect,
		Items: []model.RollbackItem{
			{
				Name:          "my-stack-web-1",
				ServiceName:   "web",
				RollbackImage: "example/web:old",
				Config: model.ContainerConfig{
					Networks: []string{"my-net"},
				},
			},
		},
	}

	actions, warnings := BuildRollbackPlan(manifest, model.RollbackRestoreStandard)
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}
	if len(actions) != 6 {
		t.Fatalf("expected 6 actions, got %d", len(actions))
	}
}

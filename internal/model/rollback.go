package model

import "time"

type RollbackTargetType string

const (
	RollbackTargetContainer RollbackTargetType = "container"
	RollbackTargetStack     RollbackTargetType = "stack"
)

type RollbackSourceMode string

const (
	RollbackSourceCompose RollbackSourceMode = "compose"
	RollbackSourceInspect RollbackSourceMode = "inspect"
)

type RollbackRestoreMode string

const (
	RollbackRestoreStandard RollbackRestoreMode = "standard"
	RollbackRestoreAdvanced RollbackRestoreMode = "advanced"
)

type RollbackManifest struct {
	Version      int                `json:"version"`
	TargetType   RollbackTargetType `json:"target_type"`
	TargetName   string             `json:"target_name"`
	CreatedAt    time.Time          `json:"created_at"`
	SourceMode   RollbackSourceMode `json:"source_mode"`
	ComposeFiles []string           `json:"compose_files,omitempty"`
	WorkingDir   string             `json:"working_dir,omitempty"`
	Items        []RollbackItem     `json:"items"`
	Warnings     []string           `json:"warnings,omitempty"`
}

type RollbackItem struct {
	Name          string            `json:"name"`
	ContainerID   string            `json:"container_id,omitempty"`
	ServiceName   string            `json:"service_name,omitempty"`
	CurrentImage  string            `json:"current_image"`
	RollbackImage string            `json:"rollback_image"`
	Config        ContainerConfig   `json:"config"`
	EnvMasked     map[string]string `json:"env_masked,omitempty"`
}

type RollbackPreview struct {
	Manifest    RollbackManifest `json:"manifest"`
	Yaml        string           `json:"yaml"`
	Actions     []RollbackAction `json:"actions"`
	Warnings    []string         `json:"warnings"`
	SecretsNote string           `json:"secrets_note"`
}

type RollbackAction struct {
	Type        string `json:"type"`
	Target      string `json:"target"`
	Description string `json:"description"`
	Destructive bool   `json:"destructive"`
}

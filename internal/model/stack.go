package model

type ComposeService struct {
	Name            string `json:"name"`
	Image           string `json:"image"`
	ContainerID     string `json:"container_id"`
	State           string `json:"state"`
	Status          string `json:"status"`
	Ports           []Port `json:"ports"`
	UpdateAvailable bool   `json:"update_available"`
	LocalDigest     string `json:"local_digest,omitempty"`
	RemoteDigest    string `json:"remote_digest,omitempty"`
}

type ComposeStack struct {
	Name            string           `json:"name"`
	Status          string           `json:"status"`
	ConfigFiles     []string         `json:"config_files"`
	WorkingDir      string           `json:"working_dir"`
	Services        []ComposeService `json:"services"`
	UpdateAvailable bool             `json:"update_available"`
}

type StackEvent struct {
	Type    string `json:"type"`
	Step    string `json:"step"`
	Service string `json:"service"`
	Line    string `json:"line"`
	Error   string `json:"error,omitempty"`
}

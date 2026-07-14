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
	Image           string                               `yaml:"image"`
	ContainerName   string                               `yaml:"container_name,omitempty"`
	Command         []string                             `yaml:"command,omitempty"`
	Entrypoint      []string                             `yaml:"entrypoint,omitempty"`
	User            string                               `yaml:"user,omitempty"`
	WorkingDir      string                               `yaml:"working_dir,omitempty"`
	Hostname        string                               `yaml:"hostname,omitempty"`
	DomainName      string                               `yaml:"domainname,omitempty"`
	MacAddress      string                               `yaml:"mac_address,omitempty"`
	StopSignal      string                               `yaml:"stop_signal,omitempty"`
	StopGracePeriod string                               `yaml:"stop_grace_period,omitempty"`
	Restart         string                               `yaml:"restart,omitempty"`
	Tty             bool                                 `yaml:"tty,omitempty"`
	StdinOpen       bool                                 `yaml:"stdin_open,omitempty"`
	Ports           []string                             `yaml:"ports,omitempty"`
	Volumes         []string                             `yaml:"volumes,omitempty"`
	Tmpfs           []string                             `yaml:"tmpfs,omitempty"`
	Devices         []string                             `yaml:"devices,omitempty"`
	CapAdd          []string                             `yaml:"cap_add,omitempty"`
	CapDrop         []string                             `yaml:"cap_drop,omitempty"`
	Privileged      bool                                 `yaml:"privileged,omitempty"`
	ReadOnly        bool                                 `yaml:"read_only,omitempty"`
	Init            *bool                                `yaml:"init,omitempty"`
	SecurityOpt     []string                             `yaml:"security_opt,omitempty"`
	Sysctls         map[string]string                    `yaml:"sysctls,omitempty"`
	Ulimits         map[string]composeUlimit             `yaml:"ulimits,omitempty"`
	GroupAdd        []string                             `yaml:"group_add,omitempty"`
	DNS             []string                             `yaml:"dns,omitempty"`
	DNSSearch       []string                             `yaml:"dns_search,omitempty"`
	DNSOpt          []string                             `yaml:"dns_opt,omitempty"`
	ExtraHosts      []string                             `yaml:"extra_hosts,omitempty"`
	Ipc             string                               `yaml:"ipc,omitempty"`
	Pid             string                               `yaml:"pid,omitempty"`
	Uts             string                               `yaml:"uts,omitempty"`
	ShmSize         int64                                `yaml:"shm_size,omitempty"`
	Runtime         string                               `yaml:"runtime,omitempty"`
	Environment     []string                             `yaml:"environment,omitempty"`
	Labels          map[string]string                    `yaml:"labels,omitempty"`
	Logging         *composeLogging                      `yaml:"logging,omitempty"`
	Healthcheck     *composeHealthcheck                  `yaml:"healthcheck,omitempty"`
	NetworkMode     string                               `yaml:"network_mode,omitempty"`
	Networks        map[string]*composeNetworkAttachment `yaml:"networks,omitempty"`
	MemLimit        int64                                `yaml:"mem_limit,omitempty"`
	MemReservation  int64                                `yaml:"mem_reservation,omitempty"`
	MemswapLimit    int64                                `yaml:"memswap_limit,omitempty"`
	CPUShares       int64                                `yaml:"cpu_shares,omitempty"`
	CPUs            string                               `yaml:"cpus,omitempty"`
	CpusetCpus      string                               `yaml:"cpuset,omitempty"`
	PidsLimit       int64                                `yaml:"pids_limit,omitempty"`
	OomScoreAdj     int                                  `yaml:"oom_score_adj,omitempty"`
}

type composeNetwork struct {
	External   bool              `yaml:"external,omitempty"`
	Name       string            `yaml:"name,omitempty"`
	Driver     string            `yaml:"driver,omitempty"`
	Internal   bool              `yaml:"internal,omitempty"`
	Attachable bool              `yaml:"attachable,omitempty"`
	EnableIPv6 bool              `yaml:"enable_ipv6,omitempty"`
	IPAM       *composeIPAM      `yaml:"ipam,omitempty"`
	DriverOpts map[string]string `yaml:"driver_opts,omitempty"`
	Labels     map[string]string `yaml:"labels,omitempty"`
}

type composeIPAM struct {
	Config []composeIPAMPool `yaml:"config,omitempty"`
}

type composeIPAMPool struct {
	Subnet  string `yaml:"subnet,omitempty"`
	Gateway string `yaml:"gateway,omitempty"`
	IPRange string `yaml:"ip_range,omitempty"`
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

	svc, volumes := buildComposeService(cfg, pinnedImage, includeEnv)
	networks, createCmds := buildNetworksSection(cfg, svc.Networks, "")

	cf := composeFile{
		Services: map[string]composeService{cfg.Name: svc},
	}
	if len(networks) > 0 {
		cf.Networks = networks
	}
	if len(volumes) > 0 {
		cf.Volumes = volumes
	}

	header := fmt.Sprintf("# Rollback for container: %s\n# Generated: %s\n# Previous image: %s\n%s\n",
		cfg.Name, now.Format(time.RFC3339), pinnedImage, networkCommandsHeader(createCmds))

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

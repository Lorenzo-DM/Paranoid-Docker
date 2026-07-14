package service

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"backend/internal/model"
)

type composeHealthcheck struct {
	Test          []string `yaml:"test,omitempty"`
	Interval      string   `yaml:"interval,omitempty"`
	Timeout       string   `yaml:"timeout,omitempty"`
	Retries       int      `yaml:"retries,omitempty"`
	StartPeriod   string   `yaml:"start_period,omitempty"`
	StartInterval string   `yaml:"start_interval,omitempty"`
	Disable       bool     `yaml:"disable,omitempty"`
}

type composeUlimit struct {
	Soft int64 `yaml:"soft"`
	Hard int64 `yaml:"hard"`
}

type composeLogging struct {
	Driver  string            `yaml:"driver,omitempty"`
	Options map[string]string `yaml:"options,omitempty"`
}

type composeNetworkAttachment struct {
	Aliases     []string `yaml:"aliases,omitempty"`
	IPv4Address string   `yaml:"ipv4_address,omitempty"`
	IPv6Address string   `yaml:"ipv6_address,omitempty"`
	MacAddress  string   `yaml:"mac_address,omitempty"`
}

// anonymousVolumeName matches the 64-hex names docker generates for
// anonymous volumes.
var anonymousVolumeName = regexp.MustCompile(`^[0-9a-f]{64}$`)

func formatDuration(d time.Duration) string {
	if d == 0 {
		return ""
	}
	return d.String()
}

// buildComposeService converts a captured container config into a
// compose service definition plus the named volumes it references.
// Shared by standalone-container and stack rollback emission.
func buildComposeService(cfg model.ContainerConfig, pinnedImage string, includeEnv bool) (composeService, map[string]composeVolume) {
	svc := composeService{
		Image:         pinnedImage,
		ContainerName: cfg.Name,
		Restart:       restartPolicyName(string(cfg.RestartPolicy().Name)),
	}
	volumes := map[string]composeVolume{}

	if includeEnv {
		svc.Environment = cfg.Env()
	}

	if c := cfg.Config; c != nil {
		svc.Command = c.Cmd
		svc.Entrypoint = c.Entrypoint
		svc.User = c.User
		svc.WorkingDir = c.WorkingDir
		svc.Hostname = c.Hostname
		svc.DomainName = c.Domainname
		svc.MacAddress = c.MacAddress
		svc.StopSignal = c.StopSignal
		svc.Tty = c.Tty
		svc.StdinOpen = c.OpenStdin
		if c.StopTimeout != nil {
			svc.StopGracePeriod = fmt.Sprintf("%ds", *c.StopTimeout)
		}
		if hc := c.Healthcheck; hc != nil {
			ch := &composeHealthcheck{
				Test:          hc.Test,
				Interval:      formatDuration(hc.Interval),
				Timeout:       formatDuration(hc.Timeout),
				Retries:       hc.Retries,
				StartPeriod:   formatDuration(hc.StartPeriod),
				StartInterval: formatDuration(hc.StartInterval),
			}
			if len(hc.Test) == 1 && hc.Test[0] == "NONE" {
				ch = &composeHealthcheck{Disable: true}
			}
			svc.Healthcheck = ch
		}
	}

	if cfg.RestartPolicy().Name == "on-failure" && cfg.RestartPolicy().MaximumRetryCount > 0 {
		svc.Restart = fmt.Sprintf("on-failure:%d", cfg.RestartPolicy().MaximumRetryCount)
	}

	svc.Ports = emitPorts(cfg)
	svc.Volumes, svc.Tmpfs = emitVolumes(cfg, volumes)

	if hc := cfg.HostConfig; hc != nil {
		svc.CapAdd = hc.CapAdd
		svc.CapDrop = hc.CapDrop
		svc.Privileged = hc.Privileged
		svc.ReadOnly = hc.ReadonlyRootfs
		svc.Init = hc.Init
		svc.SecurityOpt = hc.SecurityOpt
		svc.Sysctls = hc.Sysctls
		svc.GroupAdd = hc.GroupAdd
		svc.DNS = hc.DNS
		svc.DNSSearch = hc.DNSSearch
		svc.DNSOpt = hc.DNSOptions
		svc.ExtraHosts = hc.ExtraHosts
		svc.ShmSize = hc.ShmSize
		svc.Runtime = hc.Runtime
		svc.OomScoreAdj = hc.OomScoreAdj

		if hc.IpcMode != "" && hc.IpcMode != "private" {
			svc.Ipc = string(hc.IpcMode)
		}
		if hc.PidMode != "" {
			svc.Pid = string(hc.PidMode)
		}
		if hc.UTSMode != "" {
			svc.Uts = string(hc.UTSMode)
		}

		for _, dev := range hc.Devices {
			entry := dev.PathOnHost + ":" + dev.PathInContainer
			if dev.CgroupPermissions != "" && dev.CgroupPermissions != "rwm" {
				entry += ":" + dev.CgroupPermissions
			}
			svc.Devices = append(svc.Devices, entry)
		}

		if len(hc.Ulimits) > 0 {
			svc.Ulimits = map[string]composeUlimit{}
			for _, ul := range hc.Ulimits {
				svc.Ulimits[ul.Name] = composeUlimit{Soft: ul.Soft, Hard: ul.Hard}
			}
		}

		if (hc.LogConfig.Type != "" && hc.LogConfig.Type != "json-file") || len(hc.LogConfig.Config) > 0 {
			svc.Logging = &composeLogging{Driver: hc.LogConfig.Type, Options: hc.LogConfig.Config}
		}

		svc.MemLimit = hc.Memory
		svc.MemReservation = hc.MemoryReservation
		svc.MemswapLimit = hc.MemorySwap
		svc.CPUShares = hc.CPUShares
		if hc.NanoCPUs > 0 {
			svc.CPUs = fmt.Sprintf("%g", float64(hc.NanoCPUs)/1e9)
		}
		svc.CpusetCpus = hc.CpusetCpus
		if hc.PidsLimit != nil && *hc.PidsLimit > 0 {
			svc.PidsLimit = *hc.PidsLimit
		}
	}

	labels := map[string]string{}
	for k, v := range cfg.Labels() {
		if !strings.HasPrefix(k, "com.docker.compose.") {
			labels[k] = v
		}
	}
	if len(labels) > 0 {
		svc.Labels = labels
	}

	emitNetworkAttachments(cfg, &svc)

	return svc, volumes
}

func emitPorts(cfg model.ContainerConfig) []string {
	var ports []string
	for port, bindings := range cfg.PortBindings() {
		for _, b := range bindings {
			if b.HostPort == "" {
				continue
			}
			if b.HostIP != "" {
				ports = append(ports, fmt.Sprintf("%s:%s:%s/%s", b.HostIP, b.HostPort, port.Port(), port.Proto()))
			} else {
				ports = append(ports, fmt.Sprintf("%s:%s/%s", b.HostPort, port.Port(), port.Proto()))
			}
		}
	}
	sort.Strings(ports)
	return ports
}

func emitVolumes(cfg model.ContainerConfig, volumes map[string]composeVolume) (volumeEntries, tmpfsEntries []string) {
	binds := cfg.Binds()
	volumeEntries = append(volumeEntries, binds...)

	boundDests := map[string]bool{}
	for _, b := range binds {
		parts := strings.Split(b, ":")
		if len(parts) >= 2 {
			boundDests[parts[1]] = true
		}
	}

	var mountEntries []string
	for _, m := range cfg.Mounts {
		switch m.Type {
		case "volume":
			if boundDests[m.Destination] {
				// volume already covered by a bind-style entry (docker
				// lists -v name:/dest volumes in both Binds and Mounts
				// on some engine versions)
				continue
			}
			if m.Name == "" {
				continue
			}
			if anonymousVolumeName.MatchString(m.Name) {
				mountEntries = append(mountEntries, m.Destination)
				continue
			}
			entry := m.Name + ":" + m.Destination
			if !m.RW {
				entry += ":ro"
			}
			mountEntries = append(mountEntries, entry)
			volumes[m.Name] = composeVolume{External: true}
		case "bind":
			if boundDests[m.Destination] {
				continue
			}
			entry := m.Source + ":" + m.Destination
			if !m.RW {
				entry += ":ro"
			}
			mountEntries = append(mountEntries, entry)
		}
	}
	sort.Strings(mountEntries)
	volumeEntries = append(volumeEntries, mountEntries...)

	if cfg.HostConfig != nil {
		for path, opts := range cfg.HostConfig.Tmpfs {
			if opts != "" {
				tmpfsEntries = append(tmpfsEntries, path+":"+opts)
			} else {
				tmpfsEntries = append(tmpfsEntries, path)
			}
		}
		sort.Strings(tmpfsEntries)
	}
	return volumeEntries, tmpfsEntries
}

func emitNetworkAttachments(cfg model.ContainerConfig, svc *composeService) {
	mode := string(cfg.NetworkMode())
	if mode == "host" || mode == "none" || strings.HasPrefix(mode, "container:") {
		svc.NetworkMode = mode
		return
	}

	shortID := cfg.ID
	if len(shortID) > 12 {
		shortID = shortID[:12]
	}

	var names []string
	for name := range cfg.Endpoints {
		if !isDefaultNetwork(name) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return
	}

	svc.Networks = map[string]*composeNetworkAttachment{}
	for _, name := range names {
		ep := cfg.Endpoints[name]
		att := &composeNetworkAttachment{}
		if ep != nil {
			for _, alias := range ep.Aliases {
				// docker auto-injects the container short ID as an alias
				if alias == shortID || alias == cfg.Name {
					continue
				}
				att.Aliases = append(att.Aliases, alias)
			}
			if ep.IPAMConfig != nil {
				// IPAMConfig holds user-requested static addresses only;
				// runtime-assigned IPs stay out of the rollback
				att.IPv4Address = ep.IPAMConfig.IPv4Address
				att.IPv6Address = ep.IPAMConfig.IPv6Address
			}
		}
		svc.Networks[name] = att
	}
}

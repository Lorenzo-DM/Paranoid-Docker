package service

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/docker/go-units"
	"gopkg.in/yaml.v3"
)

// fullInspect is a synthetic container covering every feature the
// emitter must preserve.
func fullInspect() types.ContainerJSON {
	stopTimeout := 30
	initTrue := true
	pids := int64(200)
	return types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			ID:    "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899",
			Name:  "/fullapp",
			Image: "sha256:imgid",
			HostConfig: &container.HostConfig{
				Binds: []string{"/host/config:/etc/app:ro", "/host/data:/data"},
				PortBindings: nat.PortMap{
					"80/tcp":   []nat.PortBinding{{HostIP: "127.0.0.1", HostPort: "8080"}},
					"53/udp":   []nat.PortBinding{{HostPort: "5353"}},
					"9090/tcp": []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: "9090"}},
				},
				NetworkMode:    "appnet",
				RestartPolicy:  container.RestartPolicy{Name: "on-failure", MaximumRetryCount: 3},
				CapAdd:         []string{"NET_ADMIN"},
				CapDrop:        []string{"MKNOD"},
				SecurityOpt:    []string{"no-new-privileges:true"},
				Sysctls:        map[string]string{"net.ipv4.ip_forward": "1"},
				GroupAdd:       []string{"video"},
				DNS:            []string{"1.1.1.1"},
				DNSSearch:      []string{"example.internal"},
				ExtraHosts:     []string{"db.local:10.0.0.5"},
				ShmSize:        67108864,
				Tmpfs:          map[string]string{"/run": "size=64m", "/tmp": ""},
				ReadonlyRootfs: true,
				Init:           &initTrue,
				LogConfig: container.LogConfig{
					Type:   "json-file",
					Config: map[string]string{"max-size": "10m", "max-file": "3"},
				},
				Resources: container.Resources{
					Memory:            536870912,
					MemoryReservation: 268435456,
					NanoCPUs:          1500000000,
					CPUShares:         512,
					CpusetCpus:        "0,1",
					PidsLimit:         &pids,
					Ulimits: []*units.Ulimit{
						{Name: "nofile", Soft: 1024, Hard: 4096},
					},
					Devices: []container.DeviceMapping{
						{PathOnHost: "/dev/snd", PathInContainer: "/dev/snd", CgroupPermissions: "rwm"},
					},
				},
			},
		},
		Config: &container.Config{
			Image:       "example/app:2.1",
			User:        "1000:1000",
			WorkingDir:  "/srv",
			Hostname:    "fullapp",
			Domainname:  "internal",
			StopSignal:  "SIGQUIT",
			StopTimeout: &stopTimeout,
			Cmd:         []string{"serve", "--verbose"},
			Entrypoint:  []string{"/entrypoint.sh"},
			Env:         []string{"MODE=prod", "TOKEN=secret"},
			Labels: map[string]string{
				"app.tier":                   "backend",
				"com.docker.compose.project": "ignored",
			},
			Healthcheck: &container.HealthConfig{
				Test:        []string{"CMD", "curl", "-f", "http://localhost/health"},
				Interval:    30 * time.Second,
				Timeout:     5 * time.Second,
				Retries:     3,
				StartPeriod: 90 * time.Second,
			},
		},
		Mounts: []types.MountPoint{
			{Type: "volume", Name: "appdata", Destination: "/var/lib/app", RW: true},
			{Type: "volume", Name: "cache-ro", Destination: "/cache", RW: false},
			{
				Type:        "volume",
				Name:        "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
				Destination: "/anon",
				RW:          true,
			},
			{Type: "bind", Source: "/host/extra", Destination: "/extra", RW: true},
		},
		NetworkSettings: &types.NetworkSettings{
			Networks: map[string]*network.EndpointSettings{
				"appnet": {
					Aliases: []string{"app", "aabbccddeeff"},
					IPAMConfig: &network.EndpointIPAMConfig{
						IPv4Address: "172.28.0.10",
					},
				},
				"backnet": {
					Aliases: []string{"aabbccddeeff"},
				},
			},
		},
	}
}

func TestBuildComposeServiceGolden(t *testing.T) {
	cfg := captureContainerConfig(fullInspect())
	svc, volumes := buildComposeService(cfg, "example/app@sha256:pinned", true)

	cf := composeFile{
		Services: map[string]composeService{"fullapp": svc},
		Networks: map[string]composeNetwork{
			"appnet":  {External: true},
			"backnet": {External: true},
		},
		Volumes: volumes,
	}

	got, err := yaml.Marshal(cf)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	goldenPath := filepath.Join("testdata", "full_fidelity.golden.yaml")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden (run with UPDATE_GOLDEN=1 to create): %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("emitted compose differs from golden.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestGoldenIsValidCompose validates the golden file with the real
// docker compose parser when available.
func TestGoldenIsValidCompose(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available")
	}
	goldenPath := filepath.Join("testdata", "full_fidelity.golden.yaml")
	if _, err := os.Stat(goldenPath); err != nil {
		t.Skip("golden file missing")
	}
	cmd := exec.Command("docker", "compose", "-f", goldenPath, "config", "--quiet")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker compose config rejected golden: %v\n%s", err, out)
	}
}

func TestEmitPortsHostIP(t *testing.T) {
	cfg := captureContainerConfig(fullInspect())
	ports := emitPorts(cfg)
	want := map[string]bool{
		"127.0.0.1:8080:80/tcp": true,
		"5353:53/udp":           true,
		"0.0.0.0:9090:9090/tcp": true,
	}
	if len(ports) != len(want) {
		t.Fatalf("ports = %v", ports)
	}
	for _, p := range ports {
		if !want[p] {
			t.Errorf("unexpected port entry %q", p)
		}
	}
}

func TestEmitNetworkModeSpecial(t *testing.T) {
	for _, mode := range []string{"host", "none", "container:abc"} {
		cfg := captureContainerConfig(types.ContainerJSON{
			ContainerJSONBase: &types.ContainerJSONBase{
				ID:         "aabbccddeeff",
				Name:       "/x",
				HostConfig: &container.HostConfig{NetworkMode: container.NetworkMode(mode)},
			},
			Config: &container.Config{Image: "a:1"},
		})
		svc, _ := buildComposeService(cfg, "a:1", false)
		if svc.NetworkMode != mode {
			t.Errorf("mode %q: NetworkMode = %q", mode, svc.NetworkMode)
		}
		if len(svc.Networks) != 0 {
			t.Errorf("mode %q: networks emitted: %v", mode, svc.Networks)
		}
	}
}

func TestHealthcheckNone(t *testing.T) {
	insp := fullInspect()
	insp.Config.Healthcheck = &container.HealthConfig{Test: []string{"NONE"}}
	svc, _ := buildComposeService(captureContainerConfig(insp), "a:1", false)
	if svc.Healthcheck == nil || !svc.Healthcheck.Disable {
		t.Errorf("healthcheck = %+v, want disable", svc.Healthcheck)
	}
}

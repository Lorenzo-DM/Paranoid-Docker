package service

import (
	"context"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
)

func multiNetInspect() types.ContainerJSON {
	return types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			ID:   "aabbccddeeff00112233",
			Name: "/app",
			HostConfig: &container.HostConfig{
				NetworkMode: "frontnet",
				Binds:       []string{"/host:/data"},
			},
		},
		Config: &container.Config{Image: "app:1", Env: []string{"A=1"}},
		Mounts: []types.MountPoint{
			{Type: "bind", Source: "/host", Destination: "/data", RW: true},
			{Type: "volume", Name: "dbvol", Destination: "/var/lib/db", RW: true},
		},
		NetworkSettings: &types.NetworkSettings{
			Networks: map[string]*network.EndpointSettings{
				"frontnet": {
					Aliases:    []string{"app", "aabbccddeeff"},
					IPAMConfig: &network.EndpointIPAMConfig{IPv4Address: "172.30.0.5"},
				},
				"backnet": {
					Aliases: []string{"app-back", "aabbccddeeff"},
				},
			},
		},
	}
}

func TestRecreatePreservesEndpointsAndMounts(t *testing.T) {
	repo := newFakeRepo()
	cfg := captureContainerConfig(multiNetInspect())

	newID, err := recreateContainer(t.Context(), repo, cfg, func(string) {})
	if err != nil {
		t.Fatalf("recreateContainer: %v", err)
	}
	if newID != repo.createID {
		t.Errorf("newID = %q", newID)
	}

	// full captured Config passed through
	if repo.createdCfg != cfg.Config {
		t.Error("Config not passed through unchanged")
	}

	// named volume reconstructed as mount, bind not duplicated
	var volMounts, bindMounts int
	for _, m := range repo.createdHostCfg.Mounts {
		switch {
		case m.Type == "volume" && m.Source == "dbvol" && m.Target == "/var/lib/db":
			volMounts++
		case m.Type == "bind":
			bindMounts++
		}
	}
	if volMounts != 1 {
		t.Errorf("volume mounts = %d, want 1 (%+v)", volMounts, repo.createdHostCfg.Mounts)
	}
	if bindMounts != 0 {
		t.Errorf("bind already in Binds duplicated as mount: %+v", repo.createdHostCfg.Mounts)
	}

	// primary endpoint carries captured settings
	ep := repo.createdNetCfg.EndpointsConfig["frontnet"]
	if ep == nil || ep.IPAMConfig == nil || ep.IPAMConfig.IPv4Address != "172.30.0.5" {
		t.Errorf("primary endpoint = %+v, want static IP preserved", ep)
	}
	for _, a := range ep.Aliases {
		if a == "aabbccddeeff" {
			t.Error("auto short-ID alias not filtered")
		}
	}

	// secondary network connected BEFORE start
	calls := repo.callNames()
	connectIdx, startIdx := -1, -1
	for i, call := range calls {
		if strings.HasPrefix(call, "ConnectNetwork:backnet") {
			connectIdx = i
		}
		if strings.HasPrefix(call, "StartContainer") {
			startIdx = i
		}
	}
	if connectIdx == -1 {
		t.Fatalf("backnet never connected: %v", calls)
	}
	if startIdx == -1 || connectIdx > startIdx {
		t.Errorf("network connected after start: %v", calls)
	}
}

func TestRecreateStopFailureAborts(t *testing.T) {
	repo := newFakeRepo()
	repo.stopErr = context.DeadlineExceeded
	cfg := captureContainerConfig(multiNetInspect())

	if _, err := recreateContainer(t.Context(), repo, cfg, func(string) {}); err == nil {
		t.Fatal("expected error")
	}
	for _, call := range repo.callNames() {
		if strings.HasPrefix(call, "RemoveContainer") {
			t.Errorf("container removed after failed stop: %v", repo.callNames())
		}
	}
}

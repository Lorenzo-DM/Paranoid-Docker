package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"backend/internal/model"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"gopkg.in/yaml.v3"
)

func customNetworkInspect(name, project string) network.Inspect {
	labels := map[string]string{}
	if project != "" {
		labels["com.docker.compose.project"] = project
	}
	return network.Inspect{
		Name:       name,
		Driver:     "bridge",
		Internal:   false,
		Attachable: true,
		IPAM: network.IPAM{
			Config: []network.IPAMConfig{
				{Subnet: "172.28.0.0/16", Gateway: "172.28.0.1"},
			},
		},
		Options: map[string]string{"com.docker.network.bridge.name": "br-custom"},
		Labels:  labels,
	}
}

func TestNetworkCreateCommand(t *testing.T) {
	def := model.NetworkDef{
		Name:       "mynet",
		Driver:     "bridge",
		Attachable: true,
		Subnets:    []model.IPAMSubnet{{Subnet: "172.28.0.0/16", Gateway: "172.28.0.1"}},
		Options:    map[string]string{"com.docker.network.bridge.name": "br-custom"},
	}
	got := networkCreateCommand(def)
	want := "docker network create --subnet 172.28.0.0/16 --gateway 172.28.0.1 --attachable -o com.docker.network.bridge.name=br-custom mynet"
	if got != want {
		t.Errorf("networkCreateCommand:\ngot  %q\nwant %q", got, want)
	}
}

func TestStandaloneRollbackEmitsNetworkRecreateCommand(t *testing.T) {
	tmp := t.TempDir()
	w := NewRollbackWriter(tmp, &fakeRunner{})

	cfg := testContainerConfig()
	cfg.NetworkDefs = map[string]model.NetworkDef{
		"mynet": {
			Name:    "mynet",
			Driver:  "bridge",
			Subnets: []model.IPAMSubnet{{Subnet: "172.28.0.0/16"}},
		},
	}

	path, err := w.WriteRollbackCompose(cfg, "nginx@sha256:abc", false)
	if err != nil {
		t.Fatalf("WriteRollbackCompose: %v", err)
	}
	data, _ := os.ReadFile(path)
	content := string(data)

	if !strings.Contains(content, "docker network create --subnet 172.28.0.0/16 mynet") {
		t.Errorf("missing network recreate command in header:\n%s", content)
	}

	var cf composeFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		t.Fatal(err)
	}
	nw := cf.Networks["mynet"]
	if !nw.External || nw.Name != "mynet" {
		t.Errorf("network = %+v, want external with name", nw)
	}
}

func TestStackRollbackEmitsOwnedNetworkDefinition(t *testing.T) {
	tmp := t.TempDir()
	w := NewRollbackWriter(tmp, &fakeRunner{})
	repo := newFakeRepo()
	repo.inspects["c1"] = types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			ID:   "c1ffffffffffffffffff",
			Name: "/s1-web-1",
			HostConfig: &container.HostConfig{
				NetworkMode: "s1_appnet",
			},
		},
		Config: &container.Config{Image: "nginx:1.25"},
		NetworkSettings: &types.NetworkSettings{
			Networks: map[string]*network.EndpointSettings{
				"s1_appnet": {},
				"shared":    {},
			},
		},
	}
	repo.networks["s1_appnet"] = customNetworkInspect("s1_appnet", "s1")
	repo.networks["shared"] = customNetworkInspect("shared", "")

	stack := model.ComposeStack{
		Name: "s1",
		Services: []model.ComposeService{
			{Name: "web", ContainerID: "c1", Image: "nginx:1.25", LocalDigest: "sha256:abc"},
		},
	}

	dir, err := w.WriteStackRollback(t.Context(), stack, repo, false, "inspect")
	if err != nil {
		t.Fatalf("WriteStackRollback: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "docker-compose.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	var cf composeFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		t.Fatal(err)
	}

	owned := cf.Networks["s1_appnet"]
	if owned.External {
		t.Errorf("project-owned network emitted as external: %+v", owned)
	}
	if owned.Driver != "bridge" || owned.IPAM == nil || len(owned.IPAM.Config) != 1 || owned.IPAM.Config[0].Subnet != "172.28.0.0/16" {
		t.Errorf("owned network definition incomplete: %+v", owned)
	}
	if _, leaked := owned.Labels["com.docker.compose.project"]; leaked {
		t.Errorf("compose-internal label leaked: %+v", owned.Labels)
	}

	shared := cf.Networks["shared"]
	if !shared.External || shared.Name != "shared" {
		t.Errorf("foreign network = %+v, want external", shared)
	}
	if !strings.Contains(content, "docker network create") || !strings.Contains(content, " shared") {
		t.Errorf("missing recreate command for foreign network:\n%s", content)
	}
}

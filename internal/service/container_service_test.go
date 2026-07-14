package service

import (
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
)

func testInspect() types.ContainerJSON {
	return types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			Name:  "/myapp",
			Image: "sha256:imgid",
			HostConfig: &container.HostConfig{
				RestartPolicy: container.RestartPolicy{Name: "always"},
			},
		},
		Config: &container.Config{
			Image:  "nginx:1.25",
			Labels: map[string]string{},
		},
		NetworkSettings: &types.NetworkSettings{},
	}
}

func drainPullEvents(ch chan PullEvent) []PullEvent {
	var events []PullEvent
	for evt := range ch {
		events = append(events, evt)
	}
	return events
}

func TestUpdateContainerHappyPath(t *testing.T) {
	repo := newFakeRepo()
	repo.inspects["cid1"] = testInspect()
	repo.imageInspects["sha256:imgid"] = types.ImageInspect{
		RepoDigests: []string{"nginx@sha256:localdigest"},
	}
	repo.pullBody = `{"status":"Pulling from library/nginx"}
{"status":"Pull complete"}
`
	rw := NewRollbackWriter(t.TempDir(), &fakeRunner{})
	svc := NewContainerService(repo, NewDigestChecker(repo), rw)

	ch := make(chan PullEvent, 256)
	if err := svc.UpdateContainer(t.Context(), "cid1", ch, false); err != nil {
		t.Fatalf("UpdateContainer: %v", err)
	}

	events := drainPullEvents(ch)
	if len(events) == 0 {
		t.Fatal("no events emitted")
	}
	last := events[len(events)-1]
	if last.Type != "done" {
		t.Errorf("last event = %+v, want done", last)
	}

	// verify lifecycle order: stop → remove → create → start
	var order []string
	for _, call := range repo.callNames() {
		switch {
		case strings.HasPrefix(call, "StopContainer"),
			strings.HasPrefix(call, "RemoveContainer"),
			strings.HasPrefix(call, "CreateContainer"),
			strings.HasPrefix(call, "StartContainer"):
			order = append(order, strings.SplitN(call, ":", 2)[0])
		}
	}
	want := []string{"StopContainer", "RemoveContainer", "CreateContainer", "StartContainer"}
	if len(order) != len(want) {
		t.Fatalf("lifecycle calls = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("lifecycle calls = %v, want %v", order, want)
		}
	}

	if repo.createdName != "myapp" {
		t.Errorf("created name = %q, want myapp", repo.createdName)
	}
	if repo.createdCfg.Image != "nginx:1.25" {
		t.Errorf("created image = %q", repo.createdCfg.Image)
	}

	// rollback file written with pinned digest
	files, err := rw.ListRollbacks("myapp")
	if err != nil || len(files) != 1 {
		t.Fatalf("rollbacks = %v, %v", files, err)
	}
	if files[0].PreviousImage != "nginx@sha256:localdigest" {
		t.Errorf("PreviousImage = %q", files[0].PreviousImage)
	}
}

func TestUpdateContainerPullFailure(t *testing.T) {
	repo := newFakeRepo()
	repo.inspects["cid1"] = testInspect()
	repo.pullBody = `{"status":"Pulling"}
{"error":"manifest unknown"}
`
	rw := NewRollbackWriter(t.TempDir(), &fakeRunner{})
	svc := NewContainerService(repo, NewDigestChecker(repo), rw)

	ch := make(chan PullEvent, 256)
	err := svc.UpdateContainer(t.Context(), "cid1", ch, false)
	if err == nil {
		t.Fatal("expected error on pull failure")
	}

	// channel must be closed even on failure
	events := drainPullEvents(ch)
	foundErr := false
	for _, evt := range events {
		if evt.Type == "error" {
			foundErr = true
		}
	}
	if !foundErr {
		t.Errorf("no error event emitted: %+v", events)
	}

	// container must not be touched after failed pull
	for _, call := range repo.callNames() {
		if strings.HasPrefix(call, "StopContainer") || strings.HasPrefix(call, "RemoveContainer") {
			t.Errorf("container touched after pull failure: %s", call)
		}
	}
}

func TestGetAllMapsContainers(t *testing.T) {
	repo := newFakeRepo()
	repo.containers = []types.Container{
		{ID: "aaaabbbbccccdddd", Names: []string{"/one"}, Image: "nginx:1.25", ImageID: "sha256:x", State: "running"},
		{ID: "eeeeffffgggghhhh", Names: []string{"/two"}, Image: "redis@sha256:pinned", State: "exited"},
	}
	svc := NewContainerService(repo, NewDigestChecker(repo), NewRollbackWriter(t.TempDir(), &fakeRunner{}))

	all, err := svc.GetAll(t.Context())
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 containers, got %d", len(all))
	}
	if all[0].Name != "one" || all[1].Name != "two" {
		t.Errorf("names = %q, %q", all[0].Name, all[1].Name)
	}
}

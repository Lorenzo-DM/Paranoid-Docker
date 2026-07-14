package service

import (
	"testing"

	"github.com/docker/docker/api/types"
)

func composeLabels(project, service string) map[string]string {
	return map[string]string{
		"com.docker.compose.project":              project,
		"com.docker.compose.service":              service,
		"com.docker.compose.project.config_files": "/stacks/" + project + "/docker-compose.yaml",
		"com.docker.compose.project.working_dir":  "/stacks/" + project,
	}
}

func newTestComposeService(repo *fakeRepo, tmp string) ComposeStackService {
	runner := &fakeRunner{}
	rw := NewRollbackWriter(tmp, runner)
	return NewComposeStackService(repo, NewDigestChecker(repo), NewImageSaverService(repo, tmp), runner, rw)
}

func TestListStacksGrouping(t *testing.T) {
	repo := newFakeRepo()
	repo.containers = []types.Container{
		{ID: "c1aaaaaaaaaaaaaa", Names: []string{"/s1-web-1"}, Image: "nginx:1.25", State: "running", Labels: composeLabels("s1", "web")},
		{ID: "c2aaaaaaaaaaaaaa", Names: []string{"/s1-db-1"}, Image: "postgres:16", State: "exited", Labels: composeLabels("s1", "db")},
		{ID: "c3aaaaaaaaaaaaaa", Names: []string{"/s2-app-1"}, Image: "redis:7", State: "running", Labels: composeLabels("s2", "app")},
		{ID: "c4aaaaaaaaaaaaaa", Names: []string{"/standalone"}, Image: "alpine:3", State: "running", Labels: map[string]string{}},
	}
	svc := newTestComposeService(repo, t.TempDir())

	stacks, err := svc.ListStacks(t.Context())
	if err != nil {
		t.Fatalf("ListStacks: %v", err)
	}
	if len(stacks) != 2 {
		t.Fatalf("expected 2 stacks, got %d: %+v", len(stacks), stacks)
	}

	byName := map[string]int{}
	for i, st := range stacks {
		byName[st.Name] = i
	}
	s1 := stacks[byName["s1"]]
	if len(s1.Services) != 2 {
		t.Errorf("s1 services = %d, want 2", len(s1.Services))
	}
	if s1.Status != "partial" {
		t.Errorf("s1 status = %q, want partial", s1.Status)
	}
	if s1.WorkingDir != "/stacks/s1" {
		t.Errorf("s1 working dir = %q", s1.WorkingDir)
	}
	if len(s1.ConfigFiles) != 1 || s1.ConfigFiles[0] != "/stacks/s1/docker-compose.yaml" {
		t.Errorf("s1 config files = %v", s1.ConfigFiles)
	}

	s2 := stacks[byName["s2"]]
	if len(s2.Services) != 1 || s2.Status != "running" {
		t.Errorf("s2 = %+v", s2)
	}
	// config file paths not accessible in test env → inspect mode under auto
	if s2.RollbackMode != "inspect" {
		t.Errorf("s2 rollback mode = %q, want inspect", s2.RollbackMode)
	}
}

func TestSplitConfigFiles(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"/a/compose.yaml", 1},
		{"/a/compose.yaml,/a/override.yaml", 2},
		{" /a/compose.yaml , ", 1},
	}
	for _, tc := range cases {
		if got := splitConfigFiles(tc.input); len(got) != tc.want {
			t.Errorf("splitConfigFiles(%q) = %v, want %d entries", tc.input, got, tc.want)
		}
	}
}

func TestGetCapabilitiesAndRollbackMode(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestComposeService(repo, t.TempDir())

	caps := svc.GetCapabilities(t.Context())
	if caps.RollbackMode != "auto" {
		t.Errorf("default mode = %q, want auto", caps.RollbackMode)
	}

	svc.SetRollbackMode("inspect")
	if svc.GetRollbackMode() != "inspect" {
		t.Errorf("mode = %q after SetRollbackMode", svc.GetRollbackMode())
	}

	svc.SetRollbackMode("bogus")
	if svc.GetRollbackMode() != "inspect" {
		t.Errorf("invalid mode accepted: %q", svc.GetRollbackMode())
	}
}

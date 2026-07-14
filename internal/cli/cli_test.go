package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"backend/internal/model"
	"backend/internal/service"
)

type fakeContainers struct {
	containers []model.Container
	updateErr  error
}

func (f *fakeContainers) GetAll(ctx context.Context) ([]model.Container, error) {
	return f.containers, nil
}

func (f *fakeContainers) UpdateContainer(ctx context.Context, id string, ch chan<- service.PullEvent, includeEnv bool) error {
	defer close(ch)
	ch <- service.PullEvent{Type: "progress", Status: "Pulling..."}
	if f.updateErr != nil {
		ch <- service.PullEvent{Type: "error", Error: f.updateErr.Error()}
		return f.updateErr
	}
	ch <- service.PullEvent{Type: "done", Status: "Container updated successfully"}
	return nil
}

func (f *fakeContainers) SnapshotContainer(ctx context.Context, id string, includeEnv bool) (string, error) {
	return "rollbacks/" + id + "/file.yaml", nil
}

func (f *fakeContainers) StreamLogs(ctx context.Context, id string, w io.Writer) error { return nil }

func (f *fakeContainers) ListRollbacksForContainer(name string) ([]model.RollbackFile, error) {
	return []model.RollbackFile{{Filename: "r.yaml", CreatedAt: time.Now(), PreviousImage: "nginx@sha256:x"}}, nil
}

type fakeCompose struct {
	stacks []model.ComposeStack
}

func (f *fakeCompose) ListStacks(ctx context.Context) ([]model.ComposeStack, error) {
	return f.stacks, nil
}

func (f *fakeCompose) UpdateStack(ctx context.Context, name string, ch chan<- model.StackEvent, includeEnv bool) error {
	defer close(ch)
	ch <- model.StackEvent{Type: "progress", Step: "pull", Line: "pulling images"}
	ch <- model.StackEvent{Type: "done", Line: "Stack updated successfully"}
	return nil
}

func (f *fakeCompose) SaveStackImages(ctx context.Context, name string, ch chan<- model.StackEvent) error {
	defer close(ch)
	ch <- model.StackEvent{Type: "done", Line: "saved"}
	return nil
}

func (f *fakeCompose) SaveAndUpdateStack(ctx context.Context, name string, ch chan<- model.StackEvent, includeEnv bool) error {
	defer close(ch)
	ch <- model.StackEvent{Type: "done", Line: "saved and updated"}
	return nil
}

func (f *fakeCompose) SnapshotStack(ctx context.Context, name string, includeEnv bool) (string, error) {
	return "rollbacks/" + name + "/ts", nil
}

func (f *fakeCompose) StreamStackLogs(ctx context.Context, name string, w io.Writer) error {
	return nil
}

func (f *fakeCompose) ListRollbacksForStack(name string) ([]model.RollbackFile, error) {
	return []model.RollbackFile{}, nil
}

func (f *fakeCompose) GetCapabilities(ctx context.Context) model.Capabilities {
	return model.Capabilities{}
}

func (f *fakeCompose) GetRollbackMode() string  { return "auto" }
func (f *fakeCompose) SetRollbackMode(m string) {}

type fakeSaver struct{}

func (f *fakeSaver) SaveImage(ctx context.Context, id string, ch chan<- service.SaveProgress) error {
	defer close(ch)
	ch <- service.SaveProgress{Type: "done", Filename: "img.tar.gz", SizeBytes: 42}
	return nil
}

func (f *fakeSaver) ListSavedImages() ([]model.SavedImage, error) {
	return nil, nil
}

func runCLI(t *testing.T, app *App, args ...string) (string, error) {
	t.Helper()
	out := &bytes.Buffer{}
	app.Out = out
	root := NewRootCmd(func(o Options) (*App, error) { return app, nil })
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

func testApp() *App {
	return &App{
		Containers: &fakeContainers{containers: []model.Container{
			{ShortID: "abc123def456", Name: "standalone", Image: "nginx:1.25", State: "running", UpdateAvailable: true},
			{ShortID: "fff", Name: "composed", Image: "x", ComposeManagedFilter: true},
		}},
		Compose: &fakeCompose{stacks: []model.ComposeStack{
			{Name: "s1", Status: "running", Services: []model.ComposeService{{Name: "web", Image: "nginx:1.25"}}},
		}},
		Saver: &fakeSaver{},
	}
}

func TestListStacks(t *testing.T) {
	out, err := runCLI(t, testApp(), "list", "stacks")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out, "s1") || !strings.Contains(out, "running") {
		t.Errorf("output = %s", out)
	}
}

func TestListContainersFiltersCompose(t *testing.T) {
	out, err := runCLI(t, testApp(), "list", "containers")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out, "standalone") {
		t.Errorf("missing standalone: %s", out)
	}
	if strings.Contains(out, "composed") {
		t.Errorf("compose-managed leaked: %s", out)
	}
}

func TestListStacksJSON(t *testing.T) {
	out, err := runCLI(t, testApp(), "list", "stacks", "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out, `"name": "s1"`) {
		t.Errorf("json output = %s", out)
	}
}

func TestCheckExitsWithUpdates(t *testing.T) {
	out, err := runCLI(t, testApp(), "check")
	if !errors.Is(err, ErrUpdatesAvailable) {
		t.Fatalf("err = %v, want ErrUpdatesAvailable", err)
	}
	if !strings.Contains(out, "standalone") {
		t.Errorf("output = %s", out)
	}
}

func TestCheckCleanExitsZero(t *testing.T) {
	app := testApp()
	app.Containers = &fakeContainers{}
	app.Compose = &fakeCompose{}
	out, err := runCLI(t, app, "check")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(out, "up to date") {
		t.Errorf("output = %s", out)
	}
}

func TestUpdateStackDrainsEvents(t *testing.T) {
	var logged []string
	app := testApp()
	app.LogUpdate = func(kind, target, op string) { logged = append(logged, kind+"/"+target+"/"+op) }
	out, err := runCLI(t, app, "update", "stack", "s1")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out, "pulling images") || !strings.Contains(out, "Stack updated successfully") {
		t.Errorf("output = %s", out)
	}
	if len(logged) != 1 || logged[0] != "stack/s1/update" {
		t.Errorf("logged = %v", logged)
	}
}

func TestUpdateContainerFailure(t *testing.T) {
	app := testApp()
	app.Containers = &fakeContainers{updateErr: errors.New("pull failed")}
	out, err := runCLI(t, app, "update", "container", "abc")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(out, "ERROR: pull failed") {
		t.Errorf("output = %s", out)
	}
}

func TestSnapshotContainer(t *testing.T) {
	out, err := runCLI(t, testApp(), "snapshot", "container", "abc")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out, "rollbacks/abc/file.yaml") {
		t.Errorf("output = %s", out)
	}
}

func TestSaveContainer(t *testing.T) {
	out, err := runCLI(t, testApp(), "save", "container", "abc")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out, "img.tar.gz") {
		t.Errorf("output = %s", out)
	}
}

func TestRollbacksContainer(t *testing.T) {
	out, err := runCLI(t, testApp(), "rollbacks", "container", "myapp")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out, "r.yaml") {
		t.Errorf("output = %s", out)
	}
}

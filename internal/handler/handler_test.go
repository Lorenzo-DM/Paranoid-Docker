package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"backend/internal/model"
	"backend/internal/service"
	"backend/internal/store"

	"github.com/labstack/echo/v5"
)

// fakeContainerService implements service.ContainerService.
type fakeContainerService struct {
	containers []model.Container
	rollbacks  []model.RollbackFile
	updateErr  error
}

func (f *fakeContainerService) GetAll(ctx context.Context) ([]model.Container, error) {
	return f.containers, nil
}

func (f *fakeContainerService) UpdateContainer(ctx context.Context, id string, ch chan<- service.PullEvent, includeEnv bool) error {
	defer close(ch)
	if f.updateErr != nil {
		ch <- service.PullEvent{Type: "error", Error: f.updateErr.Error()}
		return f.updateErr
	}
	ch <- service.PullEvent{Type: "progress", Status: "Pulling..."}
	ch <- service.PullEvent{Type: "done", Status: "Container updated successfully"}
	return nil
}

func (f *fakeContainerService) StreamLogs(ctx context.Context, id string, w io.Writer) error {
	_, _ = io.WriteString(w, "log line\n")
	return nil
}

func (f *fakeContainerService) ListRollbacksForContainer(name string) ([]model.RollbackFile, error) {
	return f.rollbacks, nil
}

// fakeComposeService implements service.ComposeStackService.
type fakeComposeService struct {
	stacks       []model.ComposeStack
	rollbackMode string
}

func (f *fakeComposeService) ListStacks(ctx context.Context) ([]model.ComposeStack, error) {
	return f.stacks, nil
}

func (f *fakeComposeService) UpdateStack(ctx context.Context, name string, ch chan<- model.StackEvent, includeEnv bool) error {
	defer close(ch)
	ch <- model.StackEvent{Type: "progress", Step: "pull", Line: "pulling"}
	ch <- model.StackEvent{Type: "done", Line: "Stack updated successfully"}
	return nil
}

func (f *fakeComposeService) SaveStackImages(ctx context.Context, name string, ch chan<- model.StackEvent) error {
	defer close(ch)
	ch <- model.StackEvent{Type: "done", Line: "saved"}
	return nil
}

func (f *fakeComposeService) SaveAndUpdateStack(ctx context.Context, name string, ch chan<- model.StackEvent, includeEnv bool) error {
	defer close(ch)
	ch <- model.StackEvent{Type: "done", Line: "saved and updated"}
	return nil
}

func (f *fakeComposeService) SnapshotStack(ctx context.Context, name string, includeEnv bool) (string, error) {
	return "rollbacks/" + name + "/ts", nil
}

func (f *fakeComposeService) StreamStackLogs(ctx context.Context, name string, w io.Writer) error {
	return nil
}

func (f *fakeComposeService) ListRollbacksForStack(name string) ([]model.RollbackFile, error) {
	return []model.RollbackFile{}, nil
}

func (f *fakeComposeService) GetCapabilities(ctx context.Context) model.Capabilities {
	return model.Capabilities{RollbackMode: f.rollbackMode}
}

func (f *fakeComposeService) GetRollbackMode() string { return f.rollbackMode }

func (f *fakeComposeService) SetRollbackMode(mode string) { f.rollbackMode = mode }

// fakeImageSaver implements service.ImageSaverService.
type fakeImageSaver struct {
	images []model.SavedImage
}

func (f *fakeImageSaver) SaveImage(ctx context.Context, containerID string, ch chan<- service.SaveProgress) error {
	defer close(ch)
	ch <- service.SaveProgress{Type: "done", Filename: "img.tar.gz"}
	return nil
}

func (f *fakeImageSaver) ListSavedImages() ([]model.SavedImage, error) {
	return f.images, nil
}

type testEnv struct {
	e            *echo.Echo
	containerSvc *fakeContainerService
	composeSvc   *fakeComposeService
	rollbacksDir string
	imagesDir    string
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	cs := &fakeContainerService{}
	cps := &fakeComposeService{rollbackMode: "auto"}
	is := &fakeImageSaver{}
	rollbacksDir := t.TempDir()
	imagesDir := t.TempDir()

	h := NewHandler(cs, cps, is, db,
		NewJobStore(), NewSaveJobStore(), NewStackJobStore(), NewStackJobStore(), NewStackJobStore(),
		rollbacksDir, imagesDir)

	e := echo.New()
	h.RegisterRoutes(e)
	return &testEnv{e: e, containerSvc: cs, composeSvc: cps, rollbacksDir: rollbacksDir, imagesDir: imagesDir}
}

func (env *testEnv) request(method, path string, body io.Reader) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)
	return rec
}

func TestListContainersFiltersComposeManaged(t *testing.T) {
	env := newTestEnv(t)
	env.containerSvc.containers = []model.Container{
		{ID: "a", Name: "standalone", ComposeManagedFilter: false},
		{ID: "b", Name: "composed", ComposeManagedFilter: true},
	}

	rec := env.request(http.MethodGet, "/api/v1/containers", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var got []model.Container
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "standalone" {
		t.Errorf("containers = %+v, want only standalone", got)
	}
}

func TestListStacks(t *testing.T) {
	env := newTestEnv(t)
	env.composeSvc.stacks = []model.ComposeStack{{Name: "s1", Status: "running"}}

	rec := env.request(http.MethodGet, "/api/v1/stacks", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var got []model.ComposeStack
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "s1" {
		t.Errorf("stacks = %+v", got)
	}
}

func TestTriggerUpdateReturnsJobID(t *testing.T) {
	env := newTestEnv(t)

	rec := env.request(http.MethodPost, "/api/v1/containers/cid1/update", strings.NewReader(`{"include_env":true}`))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["job_id"] != "cid1" {
		t.Errorf("job_id = %q", got["job_id"])
	}
}

func TestSetRollbackModeValidation(t *testing.T) {
	env := newTestEnv(t)

	rec := env.request(http.MethodPost, "/api/v1/rollback-mode", strings.NewReader(`{"mode":"bogus"}`))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("invalid mode: status = %d, want 400", rec.Code)
	}

	rec = env.request(http.MethodPost, "/api/v1/rollback-mode", strings.NewReader(`{"mode":"inspect"}`))
	if rec.Code != http.StatusOK {
		t.Errorf("valid mode: status = %d, want 200", rec.Code)
	}
	if env.composeSvc.rollbackMode != "inspect" {
		t.Errorf("mode not applied: %q", env.composeSvc.rollbackMode)
	}
}

func TestGetCapabilities(t *testing.T) {
	env := newTestEnv(t)
	rec := env.request(http.MethodGet, "/api/v1/capabilities", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var caps model.Capabilities
	if err := json.Unmarshal(rec.Body.Bytes(), &caps); err != nil {
		t.Fatal(err)
	}
	if caps.RollbackMode != "auto" {
		t.Errorf("rollback mode = %q", caps.RollbackMode)
	}
}

func TestDownloadImage(t *testing.T) {
	env := newTestEnv(t)

	rec := env.request(http.MethodGet, "/api/v1/images/missing.tar.gz", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("missing file: status = %d, want 404", rec.Code)
	}

	if err := os.WriteFile(filepath.Join(env.imagesDir, "img.tar.gz"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	rec = env.request(http.MethodGet, "/api/v1/images/img.tar.gz", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("existing file: status = %d, want 200", rec.Code)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "img.tar.gz") {
		t.Errorf("Content-Disposition = %q", cd)
	}
}

func TestDownloadStackRollback(t *testing.T) {
	env := newTestEnv(t)

	dir := filepath.Join(env.rollbacksDir, "mystack", "ts1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yaml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rec := env.request(http.MethodGet, "/api/v1/stacks/mystack/rollbacks/ts1/docker-compose.yaml", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}

	rec = env.request(http.MethodGet, "/api/v1/stacks/mystack/rollbacks/missing/x.yaml", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("missing: status = %d, want 404", rec.Code)
	}
}

func TestPathTraversalRejected(t *testing.T) {
	env := newTestEnv(t)

	// plant a secret outside the served base dirs
	secret := filepath.Join(filepath.Dir(env.rollbacksDir), "secret.txt")
	if err := os.WriteFile(secret, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	paths := []string{
		"/api/v1/stacks/mystack/rollbacks/../../../secret.txt",
		"/api/v1/stacks/mystack/rollbacks/..%2F..%2F..%2Fsecret.txt",
		"/api/v1/stacks/../rollbacks/x.yaml",
		"/api/v1/images/..%2Fsecret.txt",
		"/api/v1/images/..%2F..%2Fsecret.txt",
		"/api/v1/containers/..%2Fx/rollbacks",
	}
	for _, p := range paths {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		rec := httptest.NewRecorder()
		env.e.ServeHTTP(rec, req)
		if rec.Code == http.StatusOK && strings.Contains(rec.Body.String(), "secret") {
			t.Errorf("%s leaked file content", p)
		}
		if rec.Code != http.StatusBadRequest && rec.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 400/404", p, rec.Code)
		}
	}
}

func TestInvalidParamsRejected(t *testing.T) {
	env := newTestEnv(t)

	cases := []struct{ method, path string }{
		{http.MethodPost, "/api/v1/stacks/bad%20name/update"},
		{http.MethodPost, "/api/v1/containers/-flag/update"},
		{http.MethodGet, "/api/v1/stacks/a%2Fb/rollbacks"},
	}
	for _, tc := range cases {
		rec := env.request(tc.method, tc.path, nil)
		if rec.Code != http.StatusBadRequest && rec.Code != http.StatusNotFound {
			t.Errorf("%s %s: status = %d, want 400", tc.method, tc.path, rec.Code)
		}
	}
}

func TestMalformedBodyRejected(t *testing.T) {
	env := newTestEnv(t)

	rec := env.request(http.MethodPost, "/api/v1/containers/cid1/update", strings.NewReader(`{"include_env": nope}`))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("malformed body: status = %d, want 400", rec.Code)
	}

	// empty body must keep working with defaults
	rec = env.request(http.MethodPost, "/api/v1/containers/cid1/update", nil)
	if rec.Code != http.StatusAccepted {
		t.Errorf("empty body: status = %d, want 202", rec.Code)
	}
}

func TestGetUpdateLog(t *testing.T) {
	env := newTestEnv(t)
	rec := env.request(http.MethodGet, "/api/v1/update-log", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
}

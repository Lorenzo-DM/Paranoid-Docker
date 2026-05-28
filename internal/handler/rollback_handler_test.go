package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"backend/internal/model"
	"backend/internal/service"

	"github.com/labstack/echo/v5"
)

type mockContainerService struct {
	service.ContainerService
	previewFunc func(ctx context.Context, name string, filename string, mode model.RollbackRestoreMode) (model.RollbackPreview, error)
	executeFunc func(ctx context.Context, name string, filename string, mode model.RollbackRestoreMode, confirmed bool, ch chan<- service.PullEvent) error
	getAllFunc  func(ctx context.Context) ([]model.Container, error)
}

func (m *mockContainerService) GetAll(ctx context.Context) ([]model.Container, error) {
	if m.getAllFunc != nil {
		return m.getAllFunc(ctx)
	}
	return []model.Container{{ID: "123", ShortID: "123", Name: "my-container"}}, nil
}

func (m *mockContainerService) PreviewContainerRollback(ctx context.Context, name string, filename string, mode model.RollbackRestoreMode) (model.RollbackPreview, error) {
	if m.previewFunc != nil {
		return m.previewFunc(ctx, name, filename, mode)
	}
	return model.RollbackPreview{}, nil
}

func (m *mockContainerService) ExecuteContainerRollback(ctx context.Context, name string, filename string, mode model.RollbackRestoreMode, confirmed bool, ch chan<- service.PullEvent) error {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, name, filename, mode, confirmed, ch)
	}
	return nil
}

type mockComposeService struct {
	service.ComposeStackService
	previewFunc func(ctx context.Context, name string, filename string, mode model.RollbackRestoreMode) (model.RollbackPreview, error)
	executeFunc func(ctx context.Context, name string, filename string, mode model.RollbackRestoreMode, confirmed bool, ch chan<- model.StackEvent) error
}

func (m *mockComposeService) PreviewStackRollback(ctx context.Context, name string, filename string, mode model.RollbackRestoreMode) (model.RollbackPreview, error) {
	if m.previewFunc != nil {
		return m.previewFunc(ctx, name, filename, mode)
	}
	return model.RollbackPreview{}, nil
}

func (m *mockComposeService) ExecuteStackRollback(ctx context.Context, name string, filename string, mode model.RollbackRestoreMode, confirmed bool, ch chan<- model.StackEvent) error {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, name, filename, mode, confirmed, ch)
	}
	return nil
}

func TestPreviewContainerRollbackHandler(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/containers/123/rollbacks/2026-05-26T15-04-05/docker-compose.rollback.yaml/preview?mode=advanced", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPathValues(echo.PathValues{
		{Name: "id", Value: "123"},
		{Name: "timestamp", Value: "2026-05-26T15-04-05"},
		{Name: "filename", Value: "docker-compose.rollback.yaml"},
	})

	calledPreview := false
	mockCS := &mockContainerService{
		previewFunc: func(ctx context.Context, name string, filename string, mode model.RollbackRestoreMode) (model.RollbackPreview, error) {
			calledPreview = true
			if name != "my-container" {
				t.Errorf("expected name = %q, got %q", "my-container", name)
			}
			if filename != "2026-05-26T15-04-05/docker-compose.rollback.yaml" {
				t.Errorf("expected filename = %q, got %q", "2026-05-26T15-04-05/docker-compose.rollback.yaml", filename)
			}
			if mode != model.RollbackRestoreAdvanced {
				t.Errorf("expected mode = %q, got %q", model.RollbackRestoreAdvanced, mode)
			}
			return model.RollbackPreview{Yaml: "services: {}"}, nil
		},
	}

	h := &Handler{
		containerService: mockCS,
	}

	if err := h.PreviewContainerRollback(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if !calledPreview {
		t.Fatalf("expected PreviewContainerRollback to be called")
	}

	var resp model.RollbackPreview
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Yaml != "services: {}" {
		t.Fatalf("unexpected Yaml in response: %q", resp.Yaml)
	}
}

func TestExecuteContainerRollbackHandlerUnconfirmed(t *testing.T) {
	e := echo.New()
	body, _ := json.Marshal(map[string]interface{}{"mode": "standard", "confirmed": false})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/containers/123/rollbacks/2026-05-26T15-04-05/docker-compose.rollback.yaml/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPathValues(echo.PathValues{
		{Name: "id", Value: "123"},
		{Name: "timestamp", Value: "2026-05-26T15-04-05"},
		{Name: "filename", Value: "docker-compose.rollback.yaml"},
	})

	mockCS := &mockContainerService{}
	h := &Handler{
		containerService: mockCS,
	}

	if err := h.ExecuteContainerRollback(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for unconfirmed execution, got %d", rec.Code)
	}
}

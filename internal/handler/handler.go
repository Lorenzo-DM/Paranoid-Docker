package handler

import (
	"backend/internal/model"
	"backend/internal/service"
	"backend/internal/store"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"
)

// validParamRe restricts user-supplied path params (container ids, stack
// names, filenames) to docker-safe characters.
var validParamRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$`)

// checkParams returns a 400 response when any param is malformed, nil
// otherwise.
func checkParams(c *echo.Context, values ...string) error {
	for _, v := range values {
		if !validParamRe.MatchString(v) {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid path parameter"})
		}
	}
	return nil
}

// withinBase reports whether p stays inside base after resolution.
func withinBase(base, p string) bool {
	rel, err := filepath.Rel(base, p)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// decodeBody decodes an optional JSON body: empty bodies leave v at its
// defaults, malformed bodies return an error.
func decodeBody(c *echo.Context, v any) error {
	err := json.NewDecoder(c.Request().Body).Decode(v)
	if err == nil || errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

type Handler struct {
	containerService        service.ContainerService
	composeService          service.ComposeStackService
	imageSaver              service.ImageSaverService
	store                   *store.Store
	jobStore                *JobBuffer[service.PullEvent]
	saveJobStore            *JobBuffer[service.SaveProgress]
	stackJobStore           *JobBuffer[model.StackEvent]
	stackSaveJobStore       *JobBuffer[model.StackEvent]
	stackSaveUpdateJobStore *JobBuffer[model.StackEvent]
	rollbacksDir            string
	imagesDir               string
}

func NewHandler(
	cs service.ContainerService,
	cps service.ComposeStackService,
	is service.ImageSaverService,
	st *store.Store,
	rollbacksDir string,
	imagesDir string,
) *Handler {
	return &Handler{
		containerService:        cs,
		composeService:          cps,
		imageSaver:              is,
		store:                   st,
		jobStore:                NewJobBuffer[service.PullEvent](),
		saveJobStore:            NewJobBuffer[service.SaveProgress](),
		stackJobStore:           NewJobBuffer[model.StackEvent](),
		stackSaveJobStore:       NewJobBuffer[model.StackEvent](),
		stackSaveUpdateJobStore: NewJobBuffer[model.StackEvent](),
		rollbacksDir:            rollbacksDir,
		imagesDir:               imagesDir,
	}
}

func (h *Handler) RegisterRoutes(e *echo.Echo) {
	api := e.Group("/api/v1")

	api.GET("/stacks", h.ListStacks)
	api.POST("/stacks/:name/update", h.TriggerStackUpdate)
	api.GET("/stacks/:name/update-status", h.StackUpdateStatus)
	api.POST("/stacks/:name/save-images", h.TriggerSaveStackImages)
	api.GET("/stacks/:name/save-status", h.StackSaveStatus)
	api.POST("/stacks/:name/save-and-update", h.TriggerSaveAndUpdate)
	api.GET("/stacks/:name/save-update-status", h.StackSaveUpdateStatus)
	api.POST("/stacks/:name/snapshot", h.SnapshotStack)
	api.GET("/stacks/:name/logs", h.StreamStackLogs)
	api.GET("/stacks/:name/rollbacks", h.ListStackRollbacks)
	api.GET("/stacks/:name/rollbacks/*", h.DownloadStackRollback)

	api.GET("/containers", h.ListContainers)
	api.POST("/containers/:id/update", h.TriggerUpdate)
	api.GET("/containers/:id/pull-status", h.PullStatus)
	api.GET("/containers/:id/logs", h.StreamLogs)
	api.GET("/containers/:id/rollbacks", h.ListRollbacks)
	api.GET("/containers/:id/rollbacks/:filename", h.DownloadRollback)

	api.POST("/containers/:id/save-image", h.TriggerSaveImage)
	api.GET("/containers/:id/save-status", h.SaveStatus)
	api.GET("/images", h.ListSavedImages)
	api.GET("/images/:filename", h.DownloadImage)

	api.GET("/capabilities", h.GetCapabilities)
	api.POST("/rollback-mode", h.SetRollbackMode)

	api.GET("/update-log", h.GetUpdateLog)
}

func (h *Handler) ListStacks(c *echo.Context) error {
	stacks, err := h.composeService.ListStacks(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if stacks == nil {
		stacks = []model.ComposeStack{}
	}
	return c.JSON(http.StatusOK, stacks)
}

type updateRequest struct {
	IncludeEnv bool `json:"include_env"`
}

func (h *Handler) TriggerStackUpdate(c *echo.Context) error {
	name := c.Param("name")
	if err := checkParams(c, name); err != nil {
		return err
	}
	var req updateRequest
	if err := decodeBody(c, &req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}

	_ = h.store.LogUpdate("stack", name, "update")

	ch := make(chan model.StackEvent, 256)
	h.stackJobStore.Start(name, ch)

	go func() {
		_ = h.composeService.UpdateStack(context.Background(), name, ch, req.IncludeEnv)
	}()

	return c.JSON(http.StatusAccepted, map[string]string{"job_id": name})
}

func (h *Handler) StackUpdateStatus(c *echo.Context) error {
	name := c.Param("name")
	if err := checkParams(c, name); err != nil {
		return err
	}
	return streamJobBuffer(c, h.stackJobStore, name, stackEventType, "no active job for stack "+name)
}

func (h *Handler) TriggerSaveStackImages(c *echo.Context) error {
	name := c.Param("name")
	if err := checkParams(c, name); err != nil {
		return err
	}

	_ = h.store.LogUpdate("stack", name, "save_images")

	ch := make(chan model.StackEvent, 256)
	h.stackSaveJobStore.Start(name, ch)

	go func() {
		_ = h.composeService.SaveStackImages(context.Background(), name, ch)
	}()

	return c.JSON(http.StatusAccepted, map[string]string{"job_id": name})
}

func (h *Handler) StackSaveStatus(c *echo.Context) error {
	name := c.Param("name")
	if err := checkParams(c, name); err != nil {
		return err
	}
	return streamJobBuffer(c, h.stackSaveJobStore, name, stackEventType, "no active job for stack "+name)
}

func (h *Handler) TriggerSaveAndUpdate(c *echo.Context) error {
	name := c.Param("name")
	if err := checkParams(c, name); err != nil {
		return err
	}
	var req updateRequest
	if err := decodeBody(c, &req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}

	_ = h.store.LogUpdate("stack", name, "save_and_update")

	ch := make(chan model.StackEvent, 256)
	h.stackSaveUpdateJobStore.Start(name, ch)

	go func() {
		_ = h.composeService.SaveAndUpdateStack(context.Background(), name, ch, req.IncludeEnv)
	}()

	return c.JSON(http.StatusAccepted, map[string]string{"job_id": name})
}

func (h *Handler) StackSaveUpdateStatus(c *echo.Context) error {
	name := c.Param("name")
	if err := checkParams(c, name); err != nil {
		return err
	}
	return streamJobBuffer(c, h.stackSaveUpdateJobStore, name, stackEventType, "no active job for stack "+name)
}

func (h *Handler) SnapshotStack(c *echo.Context) error {
	name := c.Param("name")
	if err := checkParams(c, name); err != nil {
		return err
	}
	var req updateRequest
	if err := decodeBody(c, &req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}

	dir, err := h.composeService.SnapshotStack(c.Request().Context(), name, req.IncludeEnv)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"dir": dir})
}

func stackEventType(evt model.StackEvent) string { return evt.Type }

func pullEventType(evt service.PullEvent) string { return evt.Type }

func saveEventType(evt service.SaveProgress) string { return evt.Type }

// streamJobBuffer streams a buffered job as SSE. Every event carries an
// `id:` field so the browser's automatic reconnect (Last-Event-ID)
// resumes exactly where the stream dropped; a fresh subscriber replays
// the full history first, then follows live.
func streamJobBuffer[T any](c *echo.Context, buf *JobBuffer[T], id string, typeOf func(T) string, notFound string) error {
	sseHeaders(c)
	flusher, ok := c.Response().(http.Flusher)
	if !ok {
		return echo.NewHTTPError(http.StatusInternalServerError, "streaming unsupported")
	}
	c.Response().WriteHeader(http.StatusOK)

	job, exists := buf.Get(id)
	if !exists {
		writeSSEEvent(c.Response(), "error", map[string]string{"message": notFound})
		flusher.Flush()
		return nil
	}

	idx := 0
	if lastID := c.Request().Header.Get("Last-Event-ID"); lastID != "" {
		if n, err := strconv.Atoi(lastID); err == nil && n > 0 {
			idx = n
		}
	}

	ctx := c.Request().Context()
	for {
		events, done, changed := job.snapshotAfter(idx)
		for _, evt := range events {
			idx++
			writeSSEEventWithID(c.Response(), idx, typeOf(evt), evt)
		}
		if len(events) > 0 {
			flusher.Flush()
		}
		if done {
			return nil
		}
		select {
		case <-changed:
		case <-ctx.Done():
			return nil
		}
	}
}

func (h *Handler) StreamStackLogs(c *echo.Context) error {
	name := c.Param("name")
	if err := checkParams(c, name); err != nil {
		return err
	}

	sseHeaders(c)
	flusher, ok := c.Response().(http.Flusher)
	if !ok {
		return echo.NewHTTPError(http.StatusInternalServerError, "streaming unsupported")
	}
	c.Response().WriteHeader(http.StatusOK)

	ctx := c.Request().Context()
	pr, pw := io.Pipe()

	go func() {
		err := h.composeService.StreamStackLogs(ctx, name, pw)
		pw.CloseWithError(err)
	}()

	buf := make([]byte, 4096)
	for {
		n, err := pr.Read(buf)
		if n > 0 {
			for _, line := range strings.Split(string(buf[:n]), "\n") {
				if line == "" {
					continue
				}
				writeSSEEvent(c.Response(), "log", map[string]string{"line": line})
				flusher.Flush()
			}
		}
		if err != nil {
			return nil
		}
		select {
		case <-ctx.Done():
			_ = pr.Close()
			return nil
		default:
		}
	}
}

func (h *Handler) ListStackRollbacks(c *echo.Context) error {
	name := c.Param("name")
	if err := checkParams(c, name); err != nil {
		return err
	}
	files, err := h.composeService.ListRollbacksForStack(name)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if files == nil {
		files = []model.RollbackFile{}
	}
	return c.JSON(http.StatusOK, files)
}

func (h *Handler) DownloadStackRollback(c *echo.Context) error {
	name := c.Param("name")
	if err := checkParams(c, name); err != nil {
		return err
	}
	rest := c.Param("*")
	for _, segment := range strings.Split(rest, "/") {
		if err := checkParams(c, segment); err != nil {
			return err
		}
	}
	base := filepath.Join(h.rollbacksDir, name)
	path := filepath.Join(base, filepath.Clean(rest))
	if !withinBase(base, path) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid path"})
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "file not found"})
	}
	return serveFile(c, path)
}

// serveFile serves a file from an arbitrary (possibly absolute) path.
// echo's c.File resolves against its fs.FS rooted at the working
// directory, which breaks for injected absolute dirs.
func serveFile(c *echo.Context, path string) error {
	http.ServeFile(c.Response(), c.Request(), path)
	return nil
}

func (h *Handler) ListContainers(c *echo.Context) error {
	all, err := h.containerService.GetAll(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	standalone := make([]model.Container, 0)
	for _, ct := range all {
		if !ct.ComposeManagedFilter {
			standalone = append(standalone, ct)
		}
	}
	return c.JSON(http.StatusOK, standalone)
}

func (h *Handler) TriggerUpdate(c *echo.Context) error {
	id := c.Param("id")
	if err := checkParams(c, id); err != nil {
		return err
	}
	var req updateRequest
	if err := decodeBody(c, &req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}

	if name, err := h.containerNameFromID(c, id); err == nil {
		_ = h.store.LogUpdate("container", name, "update")
	}

	ch := make(chan service.PullEvent, 128)
	h.jobStore.Start(id, ch)

	go func() {
		_ = h.containerService.UpdateContainer(context.Background(), id, ch, req.IncludeEnv)
	}()

	return c.JSON(http.StatusAccepted, map[string]string{"job_id": id})
}

func (h *Handler) PullStatus(c *echo.Context) error {
	id := c.Param("id")
	if err := checkParams(c, id); err != nil {
		return err
	}
	return streamJobBuffer(c, h.jobStore, id, pullEventType, "no active update job for this container")
}

func (h *Handler) StreamLogs(c *echo.Context) error {
	id := c.Param("id")
	if err := checkParams(c, id); err != nil {
		return err
	}

	sseHeaders(c)
	flusher, ok := c.Response().(http.Flusher)
	if !ok {
		return echo.NewHTTPError(http.StatusInternalServerError, "streaming unsupported")
	}
	c.Response().WriteHeader(http.StatusOK)

	ctx := c.Request().Context()
	pr, pw := io.Pipe()

	go func() {
		err := h.containerService.StreamLogs(ctx, id, pw)
		pw.CloseWithError(err)
	}()

	buf := make([]byte, 4096)
	for {
		n, err := pr.Read(buf)
		if n > 0 {
			for _, line := range strings.Split(string(buf[:n]), "\n") {
				if line == "" {
					continue
				}
				writeSSEEvent(c.Response(), "log", map[string]string{"line": line})
				flusher.Flush()
			}
		}
		if err != nil {
			return nil
		}
		select {
		case <-ctx.Done():
			_ = pr.Close()
			return nil
		default:
		}
	}
}

func (h *Handler) ListRollbacks(c *echo.Context) error {
	id := c.Param("id")
	if err := checkParams(c, id); err != nil {
		return err
	}
	name, err := h.containerNameFromID(c, id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	files, err := h.containerService.ListRollbacksForContainer(name)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if files == nil {
		files = []model.RollbackFile{}
	}
	return c.JSON(http.StatusOK, files)
}

func (h *Handler) DownloadRollback(c *echo.Context) error {
	id := c.Param("id")
	filename := c.Param("filename")
	if err := checkParams(c, id, filename); err != nil {
		return err
	}
	name, err := h.containerNameFromID(c, id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	path := filepath.Join(h.rollbacksDir, name, filepath.Base(filename))
	if !withinBase(h.rollbacksDir, path) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid path"})
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "file not found"})
	}
	return serveFile(c, path)
}

func (h *Handler) TriggerSaveImage(c *echo.Context) error {
	id := c.Param("id")
	if err := checkParams(c, id); err != nil {
		return err
	}

	if name, err := h.containerNameFromID(c, id); err == nil {
		_ = h.store.LogUpdate("container", name, "save_image")
	}

	ch := make(chan service.SaveProgress, 128)
	h.saveJobStore.Start(id, ch)

	go func() {
		_ = h.imageSaver.SaveImage(context.Background(), id, ch)
	}()

	return c.JSON(http.StatusAccepted, map[string]string{"job_id": id})
}

func (h *Handler) SaveStatus(c *echo.Context) error {
	id := c.Param("id")
	if err := checkParams(c, id); err != nil {
		return err
	}
	return streamJobBuffer(c, h.saveJobStore, id, saveEventType, "no active save job for this container")
}

func (h *Handler) ListSavedImages(c *echo.Context) error {
	images, err := h.imageSaver.ListSavedImages()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if images == nil {
		images = []model.SavedImage{}
	}
	return c.JSON(http.StatusOK, images)
}

func (h *Handler) DownloadImage(c *echo.Context) error {
	if err := checkParams(c, c.Param("filename")); err != nil {
		return err
	}
	filename := filepath.Base(c.Param("filename"))
	path := filepath.Join(h.imagesDir, filename)
	if !withinBase(h.imagesDir, path) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid path"})
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "file not found"})
	}
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	return serveFile(c, path)
}

func (h *Handler) GetCapabilities(c *echo.Context) error {
	caps := h.composeService.GetCapabilities(c.Request().Context())
	return c.JSON(http.StatusOK, caps)
}

type rollbackModeRequest struct {
	Mode string `json:"mode"`
}

func (h *Handler) SetRollbackMode(c *echo.Context) error {
	var req rollbackModeRequest
	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}
	if req.Mode != "auto" && req.Mode != "compose" && req.Mode != "inspect" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "mode must be auto, compose, or inspect"})
	}
	h.composeService.SetRollbackMode(req.Mode)
	return c.JSON(http.StatusOK, map[string]string{"mode": req.Mode})
}

func (h *Handler) GetUpdateLog(c *echo.Context) error {
	byTarget, err := h.store.LastUpdateByTarget()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	result := make(map[string]string, len(byTarget))
	for target, t := range byTarget {
		result[target] = t.UTC().Format("2006-01-02T15:04:05Z")
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) containerNameFromID(c *echo.Context, id string) (string, error) {
	containers, err := h.containerService.GetAll(c.Request().Context())
	if err != nil {
		return "", err
	}
	for _, ct := range containers {
		if ct.ID == id || ct.ShortID == id {
			return ct.Name, nil
		}
	}
	return id, nil
}

func sseHeaders(c *echo.Context) {
	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("X-Accel-Buffering", "no")
	c.Response().Header().Set("Connection", "keep-alive")
}

func writeSSEEvent(w io.Writer, event string, data interface{}) {
	b, _ := json.Marshal(data)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
}

func writeSSEEventWithID(w io.Writer, id int, event string, data interface{}) {
	b, _ := json.Marshal(data)
	fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", id, event, b)
}

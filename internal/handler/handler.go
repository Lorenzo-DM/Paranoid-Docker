package handler

import (
	"backend/internal/service"
	"net/http"

	"github.com/labstack/echo/v5"
)

type Handler struct {
	containerService service.ContainerService
}

func NewHandler(cs service.ContainerService) *Handler {
	return &Handler{
		containerService: cs,
	}
}

func (h *Handler) RegisterRoutes(e *echo.Echo) {
	api := e.Group("/api/v1")
	api.GET("/containers", h.ListContainers)
}

func (h *Handler) ListContainers(c *echo.Context) error {
	containers, err := h.containerService.GetAll()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, containers)
}

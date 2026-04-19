package main

import (
	"bufio"
	"net/http"
	"os"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"

	dockerclient "backend/internal/docker"
	"backend/internal/handler"
	"backend/internal/repository"
	"backend/internal/service"
)

func loadEnv() {
	f, err := os.Open(".env")
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			os.Setenv(parts[0], parts[1])
		}
	}
}

func main() {
	loadEnv()
	e := echo.New()

	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	if allowedOrigins == "" {
		allowedOrigins = "http://localhost:5173"
	}
	origins := strings.Split(allowedOrigins, ",")

	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: origins,
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodOptions},
		AllowHeaders: []string{"Content-Type"},
	}))

	dockerCli, err := dockerclient.NewClient()
	if err != nil {
		e.Logger.Error("failed to create docker client", "error", err)
		return
	}

	repo := repository.NewContainerRepository(dockerCli)
	digestChecker := service.NewDigestChecker(repo)

	imageSaverSvc := service.NewImageSaverService(repo)
	containerSvc := service.NewContainerService(repo, digestChecker)
	composeSvc := service.NewComposeStackService(repo, digestChecker, imageSaverSvc)

	jobStore := handler.NewJobStore()
	saveJobStore := handler.NewSaveJobStore()
	stackJobStore := handler.NewStackJobStore()
	stackSaveJobStore := handler.NewStackJobStore()
	stackSaveUpdateJobStore := handler.NewStackJobStore()

	h := handler.NewHandler(containerSvc, composeSvc, imageSaverSvc, jobStore, saveJobStore, stackJobStore, stackSaveJobStore, stackSaveUpdateJobStore)

	e.GET("/", func(c *echo.Context) error {
		return c.String(http.StatusOK, "Paranoid Docker Update API")
	})

	h.RegisterRoutes(e)

	if err := e.Start(":1323"); err != nil {
		e.Logger.Error("failed to start server", "error", err)
	}
}

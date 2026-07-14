package main

import (
	"bufio"
	"io/fs"
	"net/http"
	"os"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"

	dockerclient "backend/internal/docker"
	"backend/internal/handler"
	"backend/internal/repository"
	"backend/internal/service"
	"backend/internal/store"
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

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "data/history.db"
	}
	db, err := store.Open(dbPath)
	if err != nil {
		e.Logger.Error("failed to open store", "error", err)
		return
	}
	defer db.Close()

	dockerCli, err := dockerclient.NewClient()
	if err != nil {
		e.Logger.Error("failed to create docker client", "error", err)
		return
	}

	repo := repository.NewContainerRepository(dockerCli)
	digestChecker := service.NewDigestChecker(repo)

	runner := service.NewExecRunner()
	rollbackWriter := service.NewRollbackWriter("rollbacks", runner)

	imageSaverSvc := service.NewImageSaverService(repo)
	containerSvc := service.NewContainerService(repo, digestChecker, rollbackWriter)
	composeSvc := service.NewComposeStackService(repo, digestChecker, imageSaverSvc, runner, rollbackWriter)

	jobStore := handler.NewJobStore()
	saveJobStore := handler.NewSaveJobStore()
	stackJobStore := handler.NewStackJobStore()
	stackSaveJobStore := handler.NewStackJobStore()
	stackSaveUpdateJobStore := handler.NewStackJobStore()

	h := handler.NewHandler(containerSvc, composeSvc, imageSaverSvc, db, jobStore, saveJobStore, stackJobStore, stackSaveJobStore, stackSaveUpdateJobStore)

	h.RegisterRoutes(e)

	webDir := os.Getenv("WEB_DIR")
	if webDir == "" {
		webDir = "web"
	}
	if info, statErr := os.Stat(webDir); statErr == nil && info.IsDir() {
		fsys := os.DirFS(webDir)
		e.GET("/*", func(c *echo.Context) error {
			p := c.Param("*")
			if p == "" {
				p = "index.html"
			}
			if _, err := fs.Stat(fsys, p); err == nil {
				return c.FileFS(p, fsys)
			}
			return c.FileFS("index.html", fsys)
		})
	} else {
		e.GET("/", func(c *echo.Context) error {
			return c.String(http.StatusOK, "Paranoid Docker Update API")
		})
	}

	listenAddr := os.Getenv("LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = ":1323"
	}
	if err := e.Start(listenAddr); err != nil {
		e.Logger.Error("failed to start server", "error", err)
	}
}

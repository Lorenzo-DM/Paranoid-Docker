package cli

import (
	"fmt"
	"os"

	dockerclient "backend/internal/docker"
	"backend/internal/repository"
	"backend/internal/service"
	"backend/internal/store"
)

// BuildApp wires the real dependencies: docker client → repository →
// services, honoring the directory flags via the RollbackWriter.
func BuildApp(o Options) (*App, error) {
	cli, err := dockerclient.NewClient()
	if err != nil {
		return nil, fmt.Errorf("docker client: %w", err)
	}

	repo := repository.NewContainerRepository(cli)
	digestChecker := service.NewDigestChecker(repo)
	runner := service.NewExecRunner()
	rollbackWriter := service.NewRollbackWriter(o.RollbacksDir, runner)

	saver := service.NewImageSaverService(repo, o.ImagesDir)
	containers := service.NewContainerService(repo, digestChecker, rollbackWriter)
	compose := service.NewComposeStackService(repo, digestChecker, saver, runner, rollbackWriter)

	app := &App{
		Containers: containers,
		Compose:    compose,
		Saver:      saver,
	}

	if o.DBPath != "" {
		if db, err := store.Open(o.DBPath); err == nil {
			app.LogUpdate = func(kind, target, operation string) {
				_ = db.LogUpdate(kind, target, operation)
			}
		} else {
			fmt.Fprintf(os.Stderr, "warning: update log unavailable: %v\n", err)
		}
	}

	return app, nil
}

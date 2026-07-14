package cli

import (
	"errors"
	"io"

	"backend/internal/service"

	"github.com/spf13/cobra"
)

// ErrUpdatesAvailable is returned by `paranoid check` when at least one
// image has a newer remote digest; main maps it to exit code 1.
var ErrUpdatesAvailable = errors.New("updates available")

// Options carries the persistent flags shared by every command.
type Options struct {
	JSON         bool
	RollbacksDir string
	ImagesDir    string
	DBPath       string
}

// App bundles the services the commands run against; tests inject fakes.
type App struct {
	Containers service.ContainerService
	Compose    service.ComposeStackService
	Saver      service.ImageSaverService
	// LogUpdate records an operation in the update log; nil when the DB
	// is unavailable (best-effort).
	LogUpdate func(kind, target, operation string)
	Out       io.Writer
}

func (a *App) logUpdate(kind, target, operation string) {
	if a.LogUpdate != nil {
		a.LogUpdate(kind, target, operation)
	}
}

// NewRootCmd builds the CLI. build constructs the App once flags are
// parsed; tests pass a builder returning fakes.
func NewRootCmd(build func(o Options) (*App, error)) *cobra.Command {
	opts := &Options{}
	var app *App

	root := &cobra.Command{
		Use:           "paranoid",
		Short:         "Update Docker containers and compose stacks with pre-update rollbacks",
		SilenceUsage:  true,
		SilenceErrors: false,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			a, err := build(*opts)
			if err != nil {
				return err
			}
			if a.Out == nil {
				a.Out = cmd.OutOrStdout()
			}
			app = a
			return nil
		},
	}

	root.PersistentFlags().BoolVar(&opts.JSON, "json", false, "output JSON (NDJSON for streamed events)")
	root.PersistentFlags().StringVar(&opts.RollbacksDir, "rollbacks-dir", "rollbacks", "directory for rollback files")
	root.PersistentFlags().StringVar(&opts.ImagesDir, "images-dir", "images", "directory for saved image archives")
	root.PersistentFlags().StringVar(&opts.DBPath, "db", "", "path to the update-log sqlite db (best-effort)")

	getApp := func() *App { return app }
	root.AddCommand(
		newListCmd(getApp, opts),
		newCheckCmd(getApp, opts),
		newUpdateCmd(getApp, opts),
		newSnapshotCmd(getApp, opts),
		newSaveCmd(getApp, opts),
		newRollbacksCmd(getApp, opts),
	)
	return root
}

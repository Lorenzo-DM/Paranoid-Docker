package cli

import (
	"fmt"

	"backend/internal/model"
	"backend/internal/service"

	"github.com/spf13/cobra"
)

func newListCmd(app func() *App, opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List compose stacks or standalone containers",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "stacks",
			Short: "List compose stacks",
			RunE: func(cmd *cobra.Command, args []string) error {
				stacks, err := app().Compose.ListStacks(cmd.Context())
				if err != nil {
					return err
				}
				if opts.JSON {
					return printJSON(app().Out, stacks)
				}
				printStacksTable(app().Out, stacks)
				return nil
			},
		},
		&cobra.Command{
			Use:   "containers",
			Short: "List standalone containers",
			RunE: func(cmd *cobra.Command, args []string) error {
				all, err := app().Containers.GetAll(cmd.Context())
				if err != nil {
					return err
				}
				standalone := make([]model.Container, 0, len(all))
				for _, c := range all {
					if !c.ComposeManagedFilter {
						standalone = append(standalone, c)
					}
				}
				if opts.JSON {
					return printJSON(app().Out, standalone)
				}
				printContainersTable(app().Out, standalone)
				return nil
			},
		},
	)
	return cmd
}

func newCheckCmd(app func() *App, opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Check for image updates; exits 1 when updates are available (cron-friendly)",
		RunE: func(cmd *cobra.Command, args []string) error {
			type target struct {
				Kind  string `json:"kind"`
				Name  string `json:"name"`
				Image string `json:"image"`
			}
			var updates []target

			stacks, err := app().Compose.ListStacks(cmd.Context())
			if err != nil {
				return err
			}
			for _, s := range stacks {
				for _, svc := range s.Services {
					if svc.UpdateAvailable {
						updates = append(updates, target{Kind: "stack", Name: s.Name + "/" + svc.Name, Image: svc.Image})
					}
				}
			}

			all, err := app().Containers.GetAll(cmd.Context())
			if err != nil {
				return err
			}
			for _, c := range all {
				if !c.ComposeManagedFilter && c.UpdateAvailable {
					updates = append(updates, target{Kind: "container", Name: c.Name, Image: c.Image})
				}
			}

			if opts.JSON {
				if err := printJSON(app().Out, updates); err != nil {
					return err
				}
			} else if len(updates) == 0 {
				fmt.Fprintln(app().Out, "everything up to date")
			} else {
				for _, u := range updates {
					fmt.Fprintf(app().Out, "%s %s (%s)\n", u.Kind, u.Name, u.Image)
				}
			}

			if len(updates) > 0 {
				return ErrUpdatesAvailable
			}
			return nil
		},
	}
}

func newUpdateCmd(app func() *App, opts *Options) *cobra.Command {
	var includeEnv, saveFirst bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a stack or container (rollback saved first)",
	}
	cmd.PersistentFlags().BoolVar(&includeEnv, "include-env", false, "include environment variables in the rollback file")
	cmd.PersistentFlags().BoolVar(&saveFirst, "save-first", false, "save current images before updating")

	cmd.AddCommand(
		&cobra.Command{
			Use:   "stack <name>",
			Short: "Update a compose stack",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				name := args[0]
				ch := make(chan model.StackEvent, 256)
				errCh := make(chan error, 1)
				go func() {
					if saveFirst {
						app().logUpdate("stack", name, "save_and_update")
						errCh <- app().Compose.SaveAndUpdateStack(cmd.Context(), name, ch, includeEnv)
					} else {
						app().logUpdate("stack", name, "update")
						errCh <- app().Compose.UpdateStack(cmd.Context(), name, ch, includeEnv)
					}
				}()
				terminal := drainStackEvents(app().Out, ch, opts.JSON)
				if err := <-errCh; err != nil {
					return err
				}
				return terminal
			},
		},
		&cobra.Command{
			Use:   "container <id>",
			Short: "Update a standalone container",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				id := args[0]
				if saveFirst {
					app().logUpdate("container", id, "save_image")
					saveCh := make(chan service.SaveProgress, 256)
					saveErrCh := make(chan error, 1)
					go func() { saveErrCh <- app().Saver.SaveImage(cmd.Context(), id, saveCh) }()
					terminal := drainSaveEvents(app().Out, saveCh, opts.JSON)
					if err := <-saveErrCh; err != nil {
						return err
					}
					if terminal != nil {
						return terminal
					}
				}
				app().logUpdate("container", id, "update")
				ch := make(chan service.PullEvent, 256)
				errCh := make(chan error, 1)
				go func() { errCh <- app().Containers.UpdateContainer(cmd.Context(), id, ch, includeEnv) }()
				terminal := drainPullEvents(app().Out, ch, opts.JSON)
				if err := <-errCh; err != nil {
					return err
				}
				return terminal
			},
		},
	)
	return cmd
}

func newSnapshotCmd(app func() *App, opts *Options) *cobra.Command {
	var includeEnv bool

	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Write a rollback file without updating",
	}
	cmd.PersistentFlags().BoolVar(&includeEnv, "include-env", false, "include environment variables in the rollback file")

	cmd.AddCommand(
		&cobra.Command{
			Use:   "stack <name>",
			Short: "Snapshot a compose stack",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				app().logUpdate("stack", args[0], "snapshot")
				dir, err := app().Compose.SnapshotStack(cmd.Context(), args[0], includeEnv)
				if err != nil {
					return err
				}
				if opts.JSON {
					return printJSON(app().Out, map[string]string{"dir": dir})
				}
				fmt.Fprintln(app().Out, dir)
				return nil
			},
		},
		&cobra.Command{
			Use:   "container <id>",
			Short: "Snapshot a standalone container",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				app().logUpdate("container", args[0], "snapshot")
				path, err := app().Containers.SnapshotContainer(cmd.Context(), args[0], includeEnv)
				if err != nil {
					return err
				}
				if opts.JSON {
					return printJSON(app().Out, map[string]string{"path": path})
				}
				fmt.Fprintln(app().Out, path)
				return nil
			},
		},
	)
	return cmd
}

func newSaveCmd(app func() *App, opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "save",
		Short: "Save current images as .tar.gz archives",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "stack <name>",
			Short: "Save all service images of a stack",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				app().logUpdate("stack", args[0], "save_images")
				ch := make(chan model.StackEvent, 256)
				errCh := make(chan error, 1)
				go func() { errCh <- app().Compose.SaveStackImages(cmd.Context(), args[0], ch) }()
				terminal := drainStackEvents(app().Out, ch, opts.JSON)
				if err := <-errCh; err != nil {
					return err
				}
				return terminal
			},
		},
		&cobra.Command{
			Use:   "container <id>",
			Short: "Save a container's image",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				app().logUpdate("container", args[0], "save_image")
				ch := make(chan service.SaveProgress, 256)
				errCh := make(chan error, 1)
				go func() { errCh <- app().Saver.SaveImage(cmd.Context(), args[0], ch) }()
				terminal := drainSaveEvents(app().Out, ch, opts.JSON)
				if err := <-errCh; err != nil {
					return err
				}
				return terminal
			},
		},
	)
	return cmd
}

func newRollbacksCmd(app func() *App, opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rollbacks",
		Short: "List rollback files",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "stack <name>",
			Short: "List rollbacks of a stack",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				files, err := app().Compose.ListRollbacksForStack(args[0])
				if err != nil {
					return err
				}
				if opts.JSON {
					return printJSON(app().Out, files)
				}
				printRollbacksTable(app().Out, files)
				return nil
			},
		},
		&cobra.Command{
			Use:   "container <name>",
			Short: "List rollbacks of a container",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				files, err := app().Containers.ListRollbacksForContainer(args[0])
				if err != nil {
					return err
				}
				if opts.JSON {
					return printJSON(app().Out, files)
				}
				printRollbacksTable(app().Out, files)
				return nil
			},
		},
	)
	return cmd
}

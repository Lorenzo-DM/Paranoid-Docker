package main

import (
	"errors"
	"os"

	"backend/internal/cli"
)

func main() {
	root := cli.NewRootCmd(cli.BuildApp)
	if err := root.Execute(); err != nil {
		if errors.Is(err, cli.ErrUpdatesAvailable) {
			os.Exit(1)
		}
		os.Exit(2)
	}
}

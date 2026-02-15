package main

import (
	"fmt"
	"os"

	"github.com/kiddingbaby/agx/internal/app"
	"github.com/kiddingbaby/agx/internal/interfaces/cli"
)

var (
	joinArgs  = cli.JoinArgs
	escapeArg = cli.EscapeArg
)

func main() {
	container, err := app.Bootstrap()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	root := cli.New(
		container.KeyService,
		container.SessionService,
		container.LaunchService,
		cli.Handlers{
			RunDashboard:  runTUI,
			RunKeyManager: runKeyManagerTUI,
		},
	)

	os.Exit(root.Execute(os.Args[1:]))
}

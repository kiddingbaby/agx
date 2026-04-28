package main

import (
	"fmt"
	"os"

	"github.com/kiddingbaby/agx/internal/app"
	"github.com/kiddingbaby/agx/internal/interfaces/cli"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	container, err := app.Bootstrap()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	root := cli.New(
		container.ProfileService,
		cli.BuildInfo{
			Version: version,
			Commit:  commit,
			Date:    date,
		},
	)

	os.Exit(root.Execute(os.Args[1:]))
}

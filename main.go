package main

import (
	"os"

	"github.com/urfave/cli/v2"
	"my-cli/commands"
)

func main() {
	app := &cli.App{
		Name:  "my-cli",
		Usage: "A simple CLI application",
		Commands: []*cli.Command{
			commands.HelloCommand(),
		},
	}

	if err := app.Run(os.Args); err != nil {
		os.Exit(1)
	}
}

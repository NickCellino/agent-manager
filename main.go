package main

import (
	"fmt"
	"os"

	"agent-manager/commands"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:    "agent-manager",
		Usage:   "Manage opencode skills and registries",
		Version: "0.1.0",
		Commands: []*cli.Command{
			commands.SkillsCommand(),
			commands.AgentsCommand(),
			commands.RegistryCommand(),
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

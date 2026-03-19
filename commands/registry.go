package commands

import (
	"github.com/urfave/cli/v2"

	"agent-manager/internal/tui"
)

// RegistryCommand returns the registry command
func RegistryCommand() *cli.Command {
	return &cli.Command{
		Name:    "registry",
		Aliases: []string{"registries"},
		Usage:   "Manage skill registries",
		Description: `Opens an interactive TUI to manage your skill registries.

The TUI allows you to:
  - View all configured registries
  - Add new registries (GitHub or local)
  - Delete existing registries

Registry types:
  - GitHub: Format as "owner/repo" (e.g., "NickCellino/laptop-setup")
  - Local:  Absolute or home-relative path (e.g., "~/Code/skills")

Registries are stored in ~/.local/share/agent-manager/registries.json`,
		Subcommands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List all registries",
				Action: func(c *cli.Context) error {
					return tui.RunRegistryTUI()
				},
			},
		},
		Action: func(c *cli.Context) error {
			// Default action opens the TUI
			return tui.RunRegistryTUI()
		},
	}
}

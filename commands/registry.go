package commands

import (
	"fmt"

	"github.com/urfave/cli/v2"

	"agent-manager/internal/models"
	"agent-manager/internal/storage"
	"agent-manager/internal/tui"
)

// RegistryCommand returns the registry command
func RegistryCommand() *cli.Command {
	return &cli.Command{
		Name:    "registry",
		Aliases: []string{"registries"},
		Usage:   "Manage skill registries",
		Description: `Manage your skill registries. Opens an interactive TUI by default.

Use subcommands for non-interactive operation:
  list    List all configured registries
  add     Add a new registry
  remove  Remove an existing registry

Registry types:
  github  GitHub repository (format: owner/repo)
  local   Local directory (format: /absolute/path or ~/relative/path)

Registries are stored in ~/.local/share/agent-manager/registries.json`,
		Subcommands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List all configured registries",
				Action: func(c *cli.Context) error {
					store, err := storage.LoadRegistries()
					if err != nil {
						return err
					}
					if len(store.Registries) == 0 {
						fmt.Println("No registries configured.")
						fmt.Println("Add one with: agent-manager registry add <type> <location>")
						return nil
					}
					fmt.Printf("Configured registries (%d total):\n", len(store.Registries))
					for _, r := range store.Registries {
						fmt.Printf("  [%s] %s\n", r.Type, r.Location)
					}
					return nil
				},
			},
			{
				Name:      "add",
				Usage:     "Add a new registry",
				ArgsUsage: "<type> <location>",
				Description: `Add a new skill registry.

Type must be one of:
  github  GitHub repository (location format: owner/repo)
  local   Local directory (location format: /absolute/path or ~/relative/path)

Examples:
  agent-manager registry add github NickCellino/laptop-setup
  agent-manager registry add local ~/Code/skills`,
				Action: func(c *cli.Context) error {
					if c.NArg() < 2 {
						return fmt.Errorf("usage: agent-manager registry add <type> <location>")
					}
					registryType := models.RegistryType(c.Args().Get(0))
					location := c.Args().Get(1)

					if registryType != models.RegistryTypeGitHub && registryType != models.RegistryTypeLocal {
						return fmt.Errorf("invalid registry type %q: must be %q or %q", registryType, models.RegistryTypeGitHub, models.RegistryTypeLocal)
					}

					store, err := storage.LoadRegistries()
					if err != nil {
						return err
					}

					registry := models.Registry{Type: registryType, Location: location}
					if err := storage.AddRegistry(store, registry); err != nil {
						return err
					}

					fmt.Printf("Added %s registry: %s\n", registryType, location)
					return nil
				},
			},
			{
				Name:      "remove",
				Usage:     "Remove a registry",
				ArgsUsage: "<type> <location>",
				Description: `Remove an existing skill registry.

Examples:
  agent-manager registry remove github NickCellino/laptop-setup
  agent-manager registry remove local ~/Code/skills`,
				Action: func(c *cli.Context) error {
					if c.NArg() < 2 {
						return fmt.Errorf("usage: agent-manager registry remove <type> <location>")
					}
					registryType := models.RegistryType(c.Args().Get(0))
					location := c.Args().Get(1)

					store, err := storage.LoadRegistries()
					if err != nil {
						return err
					}

					if err := storage.RemoveRegistry(store, registryType, location); err != nil {
						return err
					}

					fmt.Printf("Removed %s registry: %s\n", registryType, location)
					return nil
				},
			},
		},
		Action: func(c *cli.Context) error {
			// Default action opens the TUI
			return tui.RunRegistryTUI()
		},
	}
}

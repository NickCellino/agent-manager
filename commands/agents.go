package commands

import (
	"fmt"
	"strings"

	"github.com/urfave/cli/v2"

	"agent-manager/internal/agents"
	"agent-manager/internal/models"
	"agent-manager/internal/storage"
	"agent-manager/internal/tui"
)

// AgentsCommand returns the agents command.
func AgentsCommand() *cli.Command {
	return &cli.Command{
		Name:  "agents",
		Usage: "Manage agents for the current project",
		Description: `Manage OpenCode agents for the current project. Opens an interactive TUI by default.

Use subcommands for non-interactive operation:
  list       List all available agents from configured registries
  installed  List agents currently installed in this project
  add        Add an agent to this project
  remove     Remove an agent from this project`,
		Subcommands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List all available agents from configured registries",
				Action: func(c *cli.Context) error {
					allAgents, err := agents.DiscoverAgents()
					if err != nil {
						return err
					}
					if len(allAgents) == 0 {
						fmt.Println("No agents found. Add a registry with: agent-manager registry add <type> <location>")
						return nil
					}
					fmt.Printf("Available agents (%d total):\n", len(allAgents))
					for _, a := range allAgents {
						fmt.Printf("  %s [%s: %s]\n", a.Name, a.Registry.Type, a.Registry.Location)
					}
					return nil
				},
			},
			{
				Name:  "installed",
				Usage: "List agents installed in the current project",
				Action: func(c *cli.Context) error {
					lockFile, err := storage.LoadLockFile()
					if err != nil {
						return err
					}
					if len(lockFile.Agents) == 0 {
						fmt.Println("No agents managed by agent-manager in this project.")
						return nil
					}
					fmt.Printf("Installed agents (%d total):\n", len(lockFile.Agents))
					for _, entry := range lockFile.Agents {
						line := fmt.Sprintf("  %s", entry.Name)
						if entry.InstalledPath != entry.Name {
							line += fmt.Sprintf(" (installed as '%s')", entry.InstalledPath)
						}
						line += fmt.Sprintf(" [%s: %s]", entry.Registry.Type, entry.Registry.Location)
						if len(entry.Commit) >= 8 {
							line += fmt.Sprintf(" @ %s", entry.Commit[:8])
						} else if entry.Commit != "" {
							line += fmt.Sprintf(" @ %s", entry.Commit)
						}
						fmt.Println(line)
					}
					return nil
				},
			},
			{
				Name:      "add",
				Usage:     "Add an agent to the current project",
				ArgsUsage: "<name>",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "registry",
						Usage: "Registry to install from (format: type:location, e.g. github:NickCellino/laptop-setup)",
					},
				},
				Description: `Add an agent to the current project by name.

If multiple registries contain an agent with the same name, use --registry to specify which one.

Examples:
  agent-manager agents add my-agent
  agent-manager agents add --registry github:NickCellino/laptop-setup my-agent`,
				Action: func(c *cli.Context) error {
					if c.NArg() < 1 {
						return fmt.Errorf("usage: agent-manager agents add <name>")
					}
					agentName := c.Args().Get(0)
					registryFlag := c.String("registry")

					allAgents, err := agents.DiscoverAgents()
					if err != nil {
						return err
					}

					// Filter by name
					var matching []models.Agent
					for _, a := range allAgents {
						if a.Name == agentName {
							matching = append(matching, a)
						}
					}

					if len(matching) == 0 {
						return fmt.Errorf("agent %q not found in any configured registry", agentName)
					}

					// Resolve which agent to install
					var chosen models.Agent
					if len(matching) > 1 && registryFlag == "" {
						fmt.Printf("Agent %q found in multiple registries:\n", agentName)
						for _, a := range matching {
							fmt.Printf("  %s:%s\n", a.Registry.Type, a.Registry.Location)
						}
						return fmt.Errorf("use --registry <type>:<location> to specify which registry to use")
					} else if registryFlag != "" {
						parts := strings.SplitN(registryFlag, ":", 2)
						if len(parts) != 2 {
							return fmt.Errorf("invalid registry format %q: expected type:location", registryFlag)
						}
						registryType := models.RegistryType(parts[0])
						registryLocation := parts[1]

						found := false
						for _, a := range matching {
							if a.Registry.Type == registryType && a.Registry.Location == registryLocation {
								chosen = a
								found = true
								break
							}
						}
						if !found {
							return fmt.Errorf("agent %q not found in registry %s:%s", agentName, registryType, registryLocation)
						}
					} else {
						chosen = matching[0]
					}

					lockFile, err := storage.LoadLockFile()
					if err != nil {
						return err
					}

					// Check if already installed
					if existing := storage.FindAgentLockFileEntry(lockFile, chosen.Name, chosen.Registry); existing != nil {
						fmt.Printf("Agent %q is already installed (as '%s').\n", agentName, existing.InstalledPath)
						return nil
					}

					installedPath, err := agents.AddAgentToProject(chosen, lockFile)
					if err != nil {
						return err
					}

					msg := fmt.Sprintf("Installed agent %q", agentName)
					if installedPath != agentName {
						msg += fmt.Sprintf(" (as '%s')", installedPath)
					}
					fmt.Println(msg)
					return nil
				},
			},
			{
				Name:      "remove",
				Usage:     "Remove an agent from the current project",
				ArgsUsage: "<name>",
				Description: `Remove an agent from the current project.

The <name> argument may be either the agent's original name or its installed path name.

Examples:
  agent-manager agents remove my-agent`,
				Action: func(c *cli.Context) error {
					if c.NArg() < 1 {
						return fmt.Errorf("usage: agent-manager agents remove <name>")
					}
					agentName := c.Args().Get(0)

					lockFile, err := storage.LoadLockFile()
					if err != nil {
						return err
					}

					// Find agent in lock file by name or installed path
					var entry *models.LockFileEntry
					for i, e := range lockFile.Agents {
						if e.Name == agentName || e.InstalledPath == agentName {
							entry = &lockFile.Agents[i]
							break
						}
					}

					if entry == nil {
						return fmt.Errorf("agent %q is not managed by agent-manager in this project", agentName)
					}

					if err := agents.RemoveAgentFromProject(entry, lockFile); err != nil {
						return err
					}

					fmt.Printf("Removed agent %q\n", agentName)
					return nil
				},
			},
		},
		Action: func(c *cli.Context) error {
			saved, err := tui.RunAgentsTUI()
			if err != nil {
				return err
			}

			if saved {
				lockFile, err := storage.LoadLockFile()
				if err != nil {
					return err
				}

				if len(lockFile.Agents) > 0 {
					fmt.Printf("\nAgents managed by agent-manager (%d total):\n", len(lockFile.Agents))
					for _, entry := range lockFile.Agents {
						fmt.Printf("  - %s", entry.Name)
						if entry.InstalledPath != entry.Name {
							fmt.Printf(" (installed as '%s')", entry.InstalledPath)
						}
						fmt.Printf(" [%s: %s]\n", entry.Registry.Type, entry.Registry.Location)
					}
				} else {
					fmt.Println("\nNo agents are currently managed by agent-manager.")
				}
			} else {
				fmt.Println("\nNo changes made.")
			}

			return nil
		},
	}
}

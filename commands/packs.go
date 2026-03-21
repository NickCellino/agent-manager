package commands

import (
	"fmt"

	"github.com/urfave/cli/v2"

	"agent-manager/internal/agents"
	"agent-manager/internal/models"
	"agent-manager/internal/skills"
	"agent-manager/internal/storage"
	"agent-manager/internal/tui"
)

// PacksCommand returns the packs command
func PacksCommand() *cli.Command {
	return &cli.Command{
		Name:    "pack",
		Aliases: []string{"packs"},
		Usage:   "Manage packs of skills and agents",
		Description: `Manage packs of skills and agents. Opens an interactive TUI by default.

A pack is a named collection of skills and agents that can be installed together.

Use subcommands for non-interactive operation:
  list     List all configured packs
  add      Create a new (empty) pack
  remove   Delete a pack
  update   Update a pack's skills and agents (interactive TUI)
  install  Install all skills and agents from a pack into the current project

Packs are stored in ~/.local/share/agent-manager/packs.json`,
		Subcommands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List all configured packs",
				Action: func(c *cli.Context) error {
					store, err := storage.LoadPacks()
					if err != nil {
						return err
					}
					if len(store.Packs) == 0 {
						fmt.Println("No packs configured.")
						fmt.Println("Add one with: agent-manager pack add <name>")
						return nil
					}
					fmt.Printf("Configured packs (%d total):\n", len(store.Packs))
					for _, p := range store.Packs {
						fmt.Printf("  %s (%d skills, %d agents)\n", p.Name, len(p.Skills), len(p.Agents))
					}
					return nil
				},
			},
			{
				Name:      "add",
				Usage:     "Create a new pack",
				ArgsUsage: "<name>",
				Description: `Create a new empty pack with the given name.

Use 'agent-manager pack update <name>' to add skills and agents to the pack.

Examples:
  agent-manager pack add my-pack`,
				Action: func(c *cli.Context) error {
					if c.NArg() < 1 {
						return fmt.Errorf("usage: agent-manager pack add <name>")
					}
					name := c.Args().Get(0)

					store, err := storage.LoadPacks()
					if err != nil {
						return err
					}

					pack := models.Pack{
						Name:   name,
						Skills: []models.PackItem{},
						Agents: []models.PackItem{},
					}
					if err := storage.AddPack(store, pack); err != nil {
						return err
					}

					fmt.Printf("Added pack %q\n", name)
					return nil
				},
			},
			{
				Name:      "remove",
				Usage:     "Remove a pack",
				ArgsUsage: "<name>",
				Description: `Remove an existing pack by name.

Examples:
  agent-manager pack remove my-pack`,
				Action: func(c *cli.Context) error {
					if c.NArg() < 1 {
						return fmt.Errorf("usage: agent-manager pack remove <name>")
					}
					name := c.Args().Get(0)

					store, err := storage.LoadPacks()
					if err != nil {
						return err
					}

					if err := storage.RemovePack(store, name); err != nil {
						return err
					}

					fmt.Printf("Removed pack %q\n", name)
					return nil
				},
			},
			{
				Name:      "update",
				Usage:     "Update a pack's skills and agents (interactive TUI)",
				ArgsUsage: "<name>",
				Description: `Open an interactive TUI to select which skills and agents belong to a pack.

The TUI shows a tabbed interface — one tab for skills and one for agents.
Use Tab to switch between tabs, Space to toggle selection, and Enter to save.

Examples:
  agent-manager pack update my-pack`,
				Action: func(c *cli.Context) error {
					if c.NArg() < 1 {
						return fmt.Errorf("usage: agent-manager pack update <name>")
					}
					name := c.Args().Get(0)

					store, err := storage.LoadPacks()
					if err != nil {
						return err
					}

					if storage.FindPack(store, name) == nil {
						return fmt.Errorf("pack %q not found", name)
					}

					return tui.RunPackUpdateTUI(name)
				},
			},
			{
				Name:      "install",
				Usage:     "Install all skills and agents from a pack into the current project",
				ArgsUsage: "<name>",
				Description: `Install every skill and agent listed in a pack into the current project.

Skills and agents that are already installed are skipped.

Examples:
  agent-manager pack install my-pack`,
				Action: func(c *cli.Context) error {
					if c.NArg() < 1 {
						return fmt.Errorf("usage: agent-manager pack install <name>")
					}
					name := c.Args().Get(0)

					store, err := storage.LoadPacks()
					if err != nil {
						return err
					}

					pack := storage.FindPack(store, name)
					if pack == nil {
						return fmt.Errorf("pack %q not found", name)
					}

					lockFile, err := storage.LoadLockFile()
					if err != nil {
						return err
					}

					// Discover all available skills and agents
					allSkills, err := skills.DiscoverSkills()
					if err != nil {
						return err
					}
					allAgents, err := agents.DiscoverAgents()
					if err != nil {
						return err
					}

					installedSkills := 0
					skippedSkills := 0
					for _, item := range pack.Skills {
						// Check if already installed
						if existing := storage.FindLockFileEntry(lockFile, item.Name, item.Registry); existing != nil {
							fmt.Printf("Skill %q is already installed.\n", item.Name)
							skippedSkills++
							continue
						}

						// Find the skill in discovered skills
						var found *models.Skill
						for i := range allSkills {
							if allSkills[i].Name == item.Name &&
								allSkills[i].Registry.Type == item.Registry.Type &&
								allSkills[i].Registry.Location == item.Registry.Location {
								found = &allSkills[i]
								break
							}
						}
						if found == nil {
							fmt.Printf("Warning: skill %q not found in any registry, skipping.\n", item.Name)
							continue
						}

						installedPath, err := skills.AddSkillToProject(*found, lockFile)
						if err != nil {
							fmt.Printf("Warning: failed to install skill %q: %v\n", item.Name, err)
							continue
						}
						msg := fmt.Sprintf("Installed skill %q", item.Name)
						if installedPath != item.Name {
							msg += fmt.Sprintf(" (as '%s')", installedPath)
						}
						fmt.Println(msg)
						installedSkills++
					}

					installedAgents := 0
					skippedAgents := 0
					for _, item := range pack.Agents {
						// Check if already installed
						if existing := storage.FindAgentLockFileEntry(lockFile, item.Name, item.Registry); existing != nil {
							fmt.Printf("Agent %q is already installed.\n", item.Name)
							skippedAgents++
							continue
						}

						// Find the agent in discovered agents
						var found *models.Agent
						for i := range allAgents {
							if allAgents[i].Name == item.Name &&
								allAgents[i].Registry.Type == item.Registry.Type &&
								allAgents[i].Registry.Location == item.Registry.Location {
								found = &allAgents[i]
								break
							}
						}
						if found == nil {
							fmt.Printf("Warning: agent %q not found in any registry, skipping.\n", item.Name)
							continue
						}

						installedPath, err := agents.AddAgentToProject(*found, lockFile)
						if err != nil {
							fmt.Printf("Warning: failed to install agent %q: %v\n", item.Name, err)
							continue
						}
						msg := fmt.Sprintf("Installed agent %q", item.Name)
						if installedPath != item.Name {
							msg += fmt.Sprintf(" (as '%s')", installedPath)
						}
						fmt.Println(msg)
						installedAgents++
					}

					fmt.Printf("Pack %q installed: %d skill(s), %d agent(s) installed; %d skill(s), %d agent(s) already present.\n",
						name, installedSkills, installedAgents, skippedSkills, skippedAgents)
					return nil
				},
			},
		},
		Action: func(c *cli.Context) error {
			return tui.RunPacksTUI()
		},
	}
}

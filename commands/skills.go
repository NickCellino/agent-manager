package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/urfave/cli/v2"

	"agent-manager/internal/models"
	"agent-manager/internal/skills"
	"agent-manager/internal/storage"
	"agent-manager/internal/tui"
)

// SkillsCommand returns the skills command
func SkillsCommand() *cli.Command {
	return &cli.Command{
		Name:  "skills",
		Usage: "Manage skills for the current project",
		Description: `Manage OpenCode skills for the current project. Opens an interactive TUI by default.

Use subcommands for non-interactive operation:
  list       List all available skills from configured registries
  installed  List skills currently installed in this project
  add        Add a skill to this project
  remove     Remove a skill from this project
  update     Update installed skills from their GitHub registries`,
		Subcommands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List all available skills from configured registries",
				Action: func(c *cli.Context) error {
					allSkills, err := skills.DiscoverSkills()
					if err != nil {
						return err
					}
					if len(allSkills) == 0 {
						fmt.Println("No skills found. Add a registry with: agent-manager registry add <type> <location>")
						return nil
					}
					fmt.Printf("Available skills (%d total):\n", len(allSkills))
					for _, s := range allSkills {
						fmt.Printf("  %s [%s: %s]\n", s.Name, s.Registry.Type, s.Registry.Location)
					}
					return nil
				},
			},
			{
				Name:  "installed",
				Usage: "List skills installed in the current project",
				Action: func(c *cli.Context) error {
					lockFile, err := storage.LoadLockFile()
					if err != nil {
						return err
					}
					if len(lockFile.Skills) == 0 {
						fmt.Println("No skills managed by agent-manager in this project.")
						return nil
					}
					fmt.Printf("Installed skills (%d total):\n", len(lockFile.Skills))
					for _, entry := range lockFile.Skills {
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
				Usage:     "Add a skill to the current project",
				ArgsUsage: "<name>",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "registry",
						Usage: "Registry to install from (format: type:location, e.g. github:darrenhinde/OpenAgentsControl)",
					},
				},
				Description: `Add a skill to the current project by name.

If multiple registries contain a skill with the same name, use --registry to specify which one.

Examples:
  agent-manager skills add my-skill
  agent-manager skills add --registry github:darrenhinde/OpenAgentsControl my-skill`,
				Action: func(c *cli.Context) error {
					if c.NArg() < 1 {
						return fmt.Errorf("usage: agent-manager skills add <name>")
					}
					skillName := c.Args().Get(0)
					registryFlag := c.String("registry")

					// Discover all skills
					allSkills, err := skills.DiscoverSkills()
					if err != nil {
						return err
					}

					// Filter by name
					var matching []models.Skill
					for _, s := range allSkills {
						if s.Name == skillName {
							matching = append(matching, s)
						}
					}

					if len(matching) == 0 {
						return fmt.Errorf("skill %q not found in any configured registry", skillName)
					}

					// Resolve which skill to install
					var chosen models.Skill
					if len(matching) > 1 && registryFlag == "" {
						fmt.Printf("Skill %q found in multiple registries:\n", skillName)
						for _, s := range matching {
							fmt.Printf("  %s:%s\n", s.Registry.Type, s.Registry.Location)
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
						for _, s := range matching {
							if s.Registry.Type == registryType && s.Registry.Location == registryLocation {
								chosen = s
								found = true
								break
							}
						}
						if !found {
							return fmt.Errorf("skill %q not found in registry %s:%s", skillName, registryType, registryLocation)
						}
					} else {
						chosen = matching[0]
					}

					// Load lock file
					lockFile, err := storage.LoadLockFile()
					if err != nil {
						return err
					}

					// Check if already installed
					if existing := storage.FindLockFileEntry(lockFile, chosen.Name, chosen.Registry); existing != nil {
						fmt.Printf("Skill %q is already installed (as '%s').\n", skillName, existing.InstalledPath)
						return nil
					}

					installedPath, err := skills.AddSkillToProject(chosen, lockFile)
					if err != nil {
						return err
					}

					msg := fmt.Sprintf("Installed skill %q", skillName)
					if installedPath != skillName {
						msg += fmt.Sprintf(" (as '%s')", installedPath)
					}
					fmt.Println(msg)
					return nil
				},
			},
			{
				Name:      "remove",
				Usage:     "Remove a skill from the current project",
				ArgsUsage: "<name>",
				Description: `Remove a skill from the current project.

The <name> argument may be either the skill's original name or its installed path name.

Examples:
  agent-manager skills remove my-skill`,
				Action: func(c *cli.Context) error {
					if c.NArg() < 1 {
						return fmt.Errorf("usage: agent-manager skills remove <name>")
					}
					skillName := c.Args().Get(0)

					// Load lock file
					lockFile, err := storage.LoadLockFile()
					if err != nil {
						return err
					}

					// Find skill in lock file by name or installed path
					var entry *models.LockFileEntry
					for i, e := range lockFile.Skills {
						if e.Name == skillName || e.InstalledPath == skillName {
							entry = &lockFile.Skills[i]
							break
						}
					}

					if entry == nil {
						return fmt.Errorf("skill %q is not managed by agent-manager in this project", skillName)
					}

					if err := skills.RemoveSkillFromProject(entry, lockFile); err != nil {
						return err
					}

					fmt.Printf("Removed skill %q\n", skillName)
					return nil
				},
			},
			{
				Name:      "info",
				Usage:     "Show detailed information about a skill",
				ArgsUsage: "<name>",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "registry",
						Usage: "Registry to get info from (format: type:location, e.g. github:darrenhinde/OpenAgentsControl)",
					},
					&cli.BoolFlag{
						Name:  "no-cache",
						Usage: "Force regeneration of the summary (ignore cache)",
					},
				},
				Description: `Show detailed information and summary about a skill.

Examples:
  agent-manager skills info my-skill
  agent-manager skills info --registry github:darrenhinde/OpenAgentsControl my-skill
  agent-manager skills info --no-cache my-skill`,
				Action: func(c *cli.Context) error {
					if c.NArg() < 1 {
						return fmt.Errorf("usage: agent-manager skills info <name>")
					}
					skillName := c.Args().Get(0)
					registryFlag := c.String("registry")
					noCache := c.Bool("no-cache")

					// Discover all skills
					allSkills, err := skills.DiscoverSkills()
					if err != nil {
						return err
					}

					// Filter by name
					var matching []models.Skill
					for _, s := range allSkills {
						if s.Name == skillName {
							matching = append(matching, s)
						}
					}

					if len(matching) == 0 {
						return fmt.Errorf("skill %q not found in any configured registry", skillName)
					}

					// Resolve which skill to use
					var chosen models.Skill
					if len(matching) > 1 && registryFlag == "" {
						fmt.Printf("Skill %q found in multiple registries:\n", skillName)
						for _, s := range matching {
							fmt.Printf("  %s:%s\n", s.Registry.Type, s.Registry.Location)
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
						for _, s := range matching {
							if s.Registry.Type == registryType && s.Registry.Location == registryLocation {
								chosen = s
								found = true
								break
							}
						}
						if !found {
							return fmt.Errorf("skill %q not found in registry %s:%s", skillName, registryType, registryLocation)
						}
					} else {
						chosen = matching[0]
					}

					// Load lock file to check for commit hash
					lockFile, err := storage.LoadLockFile()
					if err != nil {
						return err
					}

					// Check cache first unless --no-cache is set
					commit := storage.GetSkillCommit(lockFile, chosen)
					var summary string

					if !noCache {
						if cached, err := storage.GetCachedSummary(chosen, commit); err == nil && cached != nil {
							summary = cached.Summary
							fmt.Println("=== Skill Summary (cached) ===")
						}
					}

					if summary == "" {
						fmt.Println("=== Generating skill summary... ===")
						summary, err = skills.GenerateSkillSummary(chosen)
						if err != nil {
							return fmt.Errorf("failed to generate summary: %w", err)
						}

						// Cache the summary
						if err := storage.SaveSkillSummary(chosen, commit, summary); err != nil {
							fmt.Fprintf(os.Stderr, "Warning: failed to cache summary: %v\n", err)
						}
					}

					// Show skill info
					fmt.Printf("\n=== Skill: %s ===\n", chosen.Name)
					fmt.Printf("Registry: %s (%s)\n", chosen.Registry.Type, chosen.Registry.Location)
					if commit != "" {
						fmt.Printf("Commit: %s\n", commit[:8])
					}
					fmt.Printf("Source: %s\n\n", chosen.SourcePath)

					// Render with glamour
					fmt.Println("=== Summary ===")

					// Try to render with glamour for better formatting
					renderer, err := glamour.NewTermRenderer(
						glamour.WithAutoStyle(),
						glamour.WithWordWrap(100),
					)
					if err == nil {
						rendered, err := renderer.Render(summary)
						if err == nil {
							fmt.Println(rendered)
						} else {
							fmt.Println(summary)
						}
						renderer.Close()
					} else {
						fmt.Println(summary)
					}

					return nil
				},
			},
			{
				Name:      "update",
				Usage:     "Update installed skills from their GitHub registries",
				ArgsUsage: "[name]",
				Description: `Update skills installed from GitHub registries to their latest version.

If a skill name is provided, only that skill is updated.
If no name is provided, all skills from GitHub registries are updated.

Examples:
  agent-manager skills update
  agent-manager skills update my-skill`,
				Action: func(c *cli.Context) error {
					lockFile, err := storage.LoadLockFile()
					if err != nil {
						return err
					}

					// Determine which entries to update
					var toUpdate []*models.LockFileEntry
					if c.NArg() > 0 {
						skillName := c.Args().Get(0)
						for i, e := range lockFile.Skills {
							if e.Name == skillName || e.InstalledPath == skillName {
								toUpdate = append(toUpdate, &lockFile.Skills[i])
								break
							}
						}
						if len(toUpdate) == 0 {
							return fmt.Errorf("skill %q is not managed by agent-manager in this project", skillName)
						}
					} else {
						if len(lockFile.Skills) == 0 {
							fmt.Println("No skills managed by agent-manager in this project.")
							return nil
						}
						for i := range lockFile.Skills {
							if lockFile.Skills[i].Registry.Type == models.RegistryTypeGitHub {
								toUpdate = append(toUpdate, &lockFile.Skills[i])
							}
						}
						if len(toUpdate) == 0 {
							fmt.Println("No skills from GitHub registries to update.")
							return nil
						}
					}

					for _, entry := range toUpdate {
						if entry.Registry.Type != models.RegistryTypeGitHub {
							fmt.Printf("Skipping %q: not from a GitHub registry\n", entry.Name)
							continue
						}
						fmt.Printf("Updating skill %q...\n", entry.Name)
						if err := skills.UpdateSkillInProject(entry, lockFile); err != nil {
							fmt.Fprintf(os.Stderr, "Error: failed to update skill %q: %v\n", entry.Name, err)
							continue
						}
						fmt.Printf("Updated skill %q\n", entry.Name)
					}

					return nil
				},
			},
		},
		Action: func(c *cli.Context) error {
			saved, err := tui.RunSkillsTUI()
			if err != nil {
				return err
			}

			if saved {
				// Load lock file to show only managed skills
				lockFile, err := storage.LoadLockFile()
				if err != nil {
					return err
				}

				if len(lockFile.Skills) > 0 {
					fmt.Printf("\nSkills managed by agent-manager (%d total):\n", len(lockFile.Skills))
					for _, entry := range lockFile.Skills {
						fmt.Printf("  - %s", entry.Name)
						if entry.InstalledPath != entry.Name {
							fmt.Printf(" (installed as '%s')", entry.InstalledPath)
						}
						fmt.Printf(" [%s: %s]\n", entry.Registry.Type, entry.Registry.Location)
					}
				} else {
					fmt.Println("\nNo skills are currently managed by agent-manager.")
				}
			} else {
				fmt.Println("\nNo changes made.")
			}

			return nil
		},
	}
}

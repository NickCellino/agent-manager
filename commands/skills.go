package commands

import (
	"fmt"

	"github.com/urfave/cli/v2"

	"agent-manager/internal/storage"
	"agent-manager/internal/tui"
)

// SkillsCommand returns the skills command
func SkillsCommand() *cli.Command {
	return &cli.Command{
		Name:  "skills",
		Usage: "Manage skills for the current project",
		Description: `Opens an interactive TUI to select which skills to include in the current project.

The TUI displays all skills from your configured registries and allows you to:
  - Filter skills by typing (fuzzy matching)
  - Toggle skill selection with spacebar
  - Save selections with enter
  - Cancel with escape

Selected skills will be installed to .opencode/skills/ in the current project.
Unselected registry skills will prompt for deletion confirmation.`,
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

package commands

import (
	"fmt"

	"github.com/urfave/cli/v2"

	"agent-manager/internal/skills"
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
				// List installed skills
				installed, err := skills.ListInstalledSkills()
				if err != nil {
					return err
				}

				fmt.Printf("\nSkills installed in this project (%d total):\n", len(installed))
				for _, skill := range installed {
					fmt.Printf("  - %s\n", skill)
				}
			} else {
				fmt.Println("\nNo changes made.")
			}

			return nil
		},
	}
}

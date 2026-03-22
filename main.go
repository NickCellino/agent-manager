package main

import (
	"fmt"
	"os"

	"agent-manager/commands"
	"github.com/urfave/cli/v2"
)

const customHelpTemplate = `NAME:
   {{.Name}} - {{.Usage}}

VERSION:
   {{.Version}}

USAGE:
   {{.Name}} [GLOBAL OPTIONS] [COMMAND [COMMAND OPTIONS]]

DESCRIPTION:
   A CLI tool for managing OpenCode skills and agents across projects.

   The tool provides both interactive TUI mode and command-line operations for:
   - Managing skill registries (GitHub repos and local directories)
   - Installing and managing skills for your project
   - Installing and managing agents for your project

   Configuration files are stored in ~/.local/share/agent-manager/

COMMANDS:
   registry, registries    Manage skill registries
   skills                  Manage skills for the current project
   agents                  Manage agents for the current project
   pack, packs             Manage packs of skills and agents
   help, h                 Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --help, -h              show help
   --version, -v           print the version


COMMAND REFERENCE & EXAMPLES:

REGISTRY MANAGEMENT
   List all configured registries:
      agent-manager registry list

   Add a GitHub repository as a registry:
      agent-manager registry add github owner/repo

   Add a local directory as a registry:
      agent-manager registry add local /path/to/skills
      agent-manager registry add local ~/Code/skills

   Remove a registry:
      agent-manager registry remove github owner/repo
      agent-manager registry remove local /path/to/skills

   Open interactive registry manager (TUI):
      agent-manager registry

SKILL MANAGEMENT
   List all available skills from configured registries:
      agent-manager skills list

   List skills installed in the current project:
      agent-manager skills installed

   Add a skill to the current project:
      agent-manager skills add my-skill

   Add a skill from a specific registry:
      agent-manager skills add --registry github:owner/repo my-skill

   Remove a skill from the current project:
      agent-manager skills remove my-skill

   Open interactive skills manager (TUI):
      agent-manager skills

AGENT MANAGEMENT
   List all available agents from configured registries:
      agent-manager agents list

   List agents installed in the current project:
      agent-manager agents installed

   Add an agent to the current project:
      agent-manager agents add my-agent

   Add an agent from a specific registry:
      agent-manager agents add --registry github:owner/repo my-agent

   Remove an agent from the current project:
      agent-manager agents remove my-agent

   Open interactive agents manager (TUI):
      agent-manager agents


REGISTRY TYPES:
   github                  GitHub repository (format: owner/repo)
   local                   Local directory (format: /absolute/path or ~/relative/path)

LEARN MORE:
   Use '{{.Name}} help <command>' or '{{.Name}} <command> --help' for detailed
   information about a specific command and all its subcommands and flags.

   Examples:
      agent-manager help registry
      agent-manager skills --help
      agent-manager help agents add
`

func main() {
	app := &cli.App{
		Name:                  "agent-manager",
		Usage:                 "Manage opencode skills and registries",
		Version:               "0.1.0",
		CustomAppHelpTemplate: customHelpTemplate,
		Commands: []*cli.Command{
			commands.SkillsCommand(),
			commands.AgentsCommand(),
			commands.RegistryCommand(),
			commands.PacksCommand(),
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

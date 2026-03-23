# agent-manager

A CLI tool for managing [OpenCode](https://opencode.ai) skills and agents across projects.

## What It Does

**agent-manager** lets you:
- Configure **skill registries** (GitHub repos or local directories that contain skills and agents)
- Browse, select, and install skills from those registries into your project's `.opencode/skills/` directory
- Browse, select, and install agents from those registries into your project's `.opencode/agents/` directory
- Bundle skills and agents into reusable **packs** for easy one-command installation
- Track installed skills and agents with a per-project `agent-lock.json` lock file

## Prerequisites

- Go 1.21 or later

## Installation

### Build from source

```bash
go build -o agent-manager
```

### Install to $GOPATH/bin

```bash
go install
```

## Example Workflow

Here is a complete example that adds a registry, installs a skill from it, and then installs an agent.

```bash
# 1. Add a GitHub repository as a registry
agent-manager registry add github NickCellino/laptop-setup

# 2. See what skills are available in the registry
agent-manager skills list

# 3. Install a skill into the current project
agent-manager skills add my-skill

# 4. See what agents are available in the registry
agent-manager agents list

# 5. Install an agent into the current project
agent-manager agents add my-agent
```

After these steps:
- `my-skill` is copied into `.opencode/skills/my-skill/` in your project
- `my-agent` is copied into `.opencode/agents/my-agent.md` in your project
- Both are recorded in `agent-lock.json` so they can be updated or removed later

---

## Usage

The tool has four top-level commands: `registry`, `skills`, `agents`, and `pack`. Each opens an interactive TUI by default, or you can use subcommands for non-interactive (scriptable) operation.

---

### Registry Management

Registries are the sources of skills and agents. They are stored globally in `~/.local/share/agent-manager/registries.json`.

#### Open interactive TUI

```bash
agent-manager registry
```

#### List configured registries

```bash
agent-manager registry list
```

#### Add a registry

```bash
# GitHub repository (owner/repo format)
agent-manager registry add github NickCellino/laptop-setup

# Local directory
agent-manager registry add local ~/Code/my-skills
```

#### Remove a registry

```bash
agent-manager registry remove github NickCellino/laptop-setup
agent-manager registry remove local ~/Code/my-skills
```

---

### Skills Management

Skills are per-project. Run these commands from the project root directory. Installed skills go into `.opencode/skills/` and are tracked in `agent-lock.json`.

#### Open interactive TUI (fuzzy search + multi-select)

```bash
agent-manager skills
```

#### List all available skills (from all configured registries)

```bash
agent-manager skills list
```

#### List skills installed in the current project

```bash
agent-manager skills installed
```

#### Install a skill into the current project

```bash
agent-manager skills add my-skill

# If the same skill name exists in multiple registries, specify which one:
agent-manager skills add --registry github:NickCellino/laptop-setup my-skill
agent-manager skills add --registry local:~/Code/my-skills my-skill
```

#### Remove a skill from the current project

```bash
agent-manager skills remove my-skill
```

#### Update installed skills to their latest version

```bash
# Update all skills from GitHub registries
agent-manager skills update

# Update a specific skill
agent-manager skills update my-skill
```

---

### Agents Management

Agents are per-project. Run these commands from the project root directory. Installed agents go into `.opencode/agents/` and are tracked in `agent-lock.json`.

#### Open interactive TUI (fuzzy search + multi-select)

```bash
agent-manager agents
```

#### List all available agents (from all configured registries)

```bash
agent-manager agents list
```

#### List agents installed in the current project

```bash
agent-manager agents installed
```

#### Install an agent into the current project

```bash
agent-manager agents add my-agent

# If the same agent name exists in multiple registries, specify which one:
agent-manager agents add --registry github:NickCellino/laptop-setup my-agent
agent-manager agents add --registry local:~/Code/my-agents my-agent
```

#### Remove an agent from the current project

```bash
agent-manager agents remove my-agent
```

#### Update installed agents to their latest version

```bash
# Update all agents from GitHub registries
agent-manager agents update

# Update a specific agent
agent-manager agents update my-agent
```

---

### Packs Management

Packs are named collections of skills and agents that can be installed together with a single command. They are stored globally in `~/.local/share/agent-manager/packs.json`.

#### Open interactive TUI

```bash
agent-manager pack
```

#### List all configured packs

```bash
agent-manager pack list
```

#### Create a new pack

```bash
agent-manager pack add my-pack
```

#### Edit a pack's skills and agents (interactive TUI)

```bash
agent-manager pack update my-pack
```

#### Install all skills and agents from a pack into the current project

```bash
agent-manager pack install my-pack
```

#### Remove a pack

```bash
agent-manager pack remove my-pack
```

---

## Discovery

When scanning a registry for available skills and agents, agent-manager uses different discovery mechanisms:

### Skills

Skills are discovered as subdirectories within:
- `.agents/skills/`
- `.opencode/skills/`

Each subdirectory is treated as an individual skill.

### Agents

Agents are discovered as **`.md` files** in any directory named exactly **`agents/`** (searched recursively throughout the registry). The filename (without the `.md` extension) becomes the agent name. For example:
- `.opencode/agents/my-agent.md` → agent named "my-agent"
- `deeply/nested/agents/another-agent.md` → agent named "another-agent"

Note: Files in subdirectories of `agents/` are not recognized (e.g., `agents/subdir/file.md` is ignored).

## How Skills and Agents Are Stored

- **GitHub registries** are cloned to `~/.local/share/agent-manager/github-registries/<owner>/<repo>` on first use
- Skills are discovered inside `.agents/skills/` or `.opencode/skills/` within each registry
- Agents are discovered as `.md` files inside any `agents/` directory within each registry
- Installed skills are copied (GitHub) or symlinked (local) into `.opencode/skills/` in your project
- Installed agents are copied (GitHub) or symlinked (local) into `.opencode/agents/` in your project
- The `agent-lock.json` file records which skills and agents are managed by agent-manager, including the source registry and git commit hash (for GitHub registries)
- Skills and agents not tracked in `agent-lock.json` are never touched by agent-manager

## Testing

Run all tests:

```bash
go test ./...
```

Run tests with verbose output:

```bash
go test -v ./...
```

Run tests with coverage:

```bash
go test -cover ./...
```

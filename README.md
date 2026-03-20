# agent-manager

A CLI tool for managing [OpenCode](https://opencode.ai) skills across projects.

## What It Does

**agent-manager** lets you:
- Configure **skill registries** (GitHub repos or local directories that contain skills)
- Browse, select, and install skills from those registries into your project's `.opencode/skills/` directory
- Track installed skills with a per-project `agent-lock.json` lock file

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

## Usage

The tool has two top-level commands: `registry` and `skills`. Each opens an interactive TUI by default, or you can use subcommands for non-interactive (scriptable) operation.

---

### Registry Management

Registries are the sources of skills. They are stored globally in `~/.local/share/agent-manager/registries.json`.

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

## How Skills Are Stored

- **GitHub registries** are cloned to `~/.local/share/agent-manager/github-registries/<owner>/<repo>` on first use
- Skills are discovered inside `.agents/skills/` or `.opencode/skills/` within each registry
- Installed skills are copied (GitHub) or symlinked (local) into `.opencode/skills/` in your project
- The `agent-lock.json` file records which skills are managed by agent-manager, including the source registry and git commit hash (for GitHub registries)
- Skills not tracked in `agent-lock.json` are never touched by agent-manager

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

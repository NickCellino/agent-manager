I want to build a CLI tool agent-manager. It should help me manage all aspects of managing my opencode setup (skills, agents, etc).

# Skill management

I want to manage all my opencode skills in registries. A registry can be either a local directory (like ~/Code/laptop-setup) or a git repository (like NickCellino/laptop-setup).

The CLI should have a helpful and informative help command with full usage examples.

## Skills

### Skill Registries

From anywhere on my system, I'd like to be able to run:
```
# Opens registry manager TUI
agent-manager skills registry
```

This TUI should let me:
1. see a list of current registries
2. add a new registry
3. delete existing registries

When adding a new registry, I should be able to choose:
* Github repo (should allow input like NickCellino/laptop-setup)
* local filepath (should allow input like ~/Code/laptop-setup/ or /Users/nicholas/Code/laptop-setup/)

This list of registries should be persisted as a json file within ~/.local/share/agent-manager/ (or please advise if you would recommend another location). I'm thinking about the XDG Base Directory specification. I'm also wondering if this would provide a good mechanism for test environments (ie to run e2e tests, we can just run everything with a different, isolated XDG_DATA_HOME so we can test the full functionality without touching the real user data).

#### Finding skills within registries

For functionality below, we will need to have the capability to "find skills within a registry". Within a registry, skills can be contained at the paths:
* .agents/skills
* .opencode/skills

Reference this guide to understand the structure of skills: https://raw.githubusercontent.com/anomalyco/opencode/8e09e8c6121f03244a1f25281b506a90bcb355d7/packages/web/src/content/docs/skills.mdx

### Adding Skills

When I am working on a certain project in a certain directory, I want to be able to choose which skills from my registries to include.

From a certain project (for example ~/Code/ai/skill-manager/), I'd like to run:

```
agent-manager skills
```

This should open up a multi-select list of all the skills contained within all my registries. I should be able to filter the list in real-time by typing which should, in real-time, filter down the list of skills to only those whose name is a match for my search query (fuzzy finding if that's easy, otherwise standard substring search is fine). I should be able to toggle whether skills are selected with <spacebar> and I should be able to "Save"/"Confirm" with <enter>..

(Possible future feature: Filter skill list by which registry the skills come from)

Once I save, this should ensure all of my selected skills are installed in the current project in .opencode/skills.
For each selected skill:
If there is already a skill installed in .opencode/skills with that name, do nothing.
If there is not, install it:
    * For skills contained within a Github repository, this will mean cloning the repository if it's not already (in ~/.local/share/agent-manager/github-registries) and then copying the skill into .opencode/skills
    * For skills contained in a local folder registry, symlink the skill to the skill on the local filesystem.

For any unselected skills: these will be deleted if the user confirms. Prompt the user with a confirmation. If they confirm, delete these skills from .opencode/skills. Otherwise, do nothing and exit.

Note: The multi-select menu will only list skills that are contained within the registries. If there are skills in .opencode/skills
that I've defined locally within this project, they will not be in the list (unless by chance there happens to be a skill with the same
name in the registries). This is fine - we do not want this CLI to manage these skills' lifecycle.

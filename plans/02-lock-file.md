I will need some intermediate layer/file to keep track of what skills have been selected by the user and are being managed by the tool.

For example, as it stands, the only way we know what skills are installed are by looking at the project's .opencode/skills folder. If there is a skill named "showboat" but then "showboat" is a skill present in multiple registries, we don't know where it came from and thus, the skill selection screen becomes confusing.

The current setup also makes it difficult to track skill "versions".

I would like to introduce a "agent-lock.json" file somewhat similar to package-lock.json in its purpose. This file should keep track of what skills are installed in this project (that are managed by agent-manager), what registry they came from, and if it is a git repo, what commit hash. Each skill entry in this file will need to also keep track of the skill's true installed folder. Usually, this should be the same as the folder in the source registry unless a skill with this name already exists at that path, in which case, we should append "-{registryname}" to the skill folder name where {registryname} is a filepath friendly version of the registry name.

agent-manager should treat this file as the source of truth for what skills are installed, but it should try to keep the actual skills up-to-date.

So just as before, we can add and remove skills. when adding a new skill, agent-manager should update agent-lock.json with the new skill and then add that skill to the filesystem. If when we try to add the skill to the filesystem, we encounter an error because something already exists at that path, show a helpful error message to the user.

when removing a skill, agent-manager should delete the skill from the filesystem, then delete it from agent-lock.json. If the skill did not actually exist in the filesystem, that's fine, just continue.

As a separate but somewhat related change, I do not want registries to have "names". They should just have types and then the identifying info (like NickCellino/laptop-setup or ~/Code/laptop-setup/). This identifying info is what should be displayed in parenthesis on the skill list page and it should be colored to provide some distinction/visual interest.

---

## Implementation Plan

### Summary
Implement `agent-lock.json` lock file to track managed skills with their registry source and commit hash, and remove registry "names" in favor of Type+Location identifiers.

### Proposed `agent-lock.json` Schema
```json
{
  "skills": [
    {
      "name": "showboat",
      "installedPath": "showboat-nickcellino-laptop-setup",
      "registry": {
        "type": "github",
        "location": "NickCellino/laptop-setup",
        "commit": "abc123def456"
      }
    },
    {
      "name": "docker-helper",
      "installedPath": "docker-helper",
      "registry": {
        "type": "local",
        "location": "~/Code/my-skills"
      }
    }
  ]
}
```

### Detailed Implementation Steps

#### Phase 1: Model Changes

**File: `internal/models/models.go`**

1. Remove `Name` field from `Registry` struct
2. Update `Skill` struct to reference `Registry` directly (not by name string)
3. Add `LockFile` struct with `LockFileEntry` array
4. Add `Registry` field inside `LockFileEntry` with optional `Commit` field

#### Phase 2: Lock File Storage (New File)

**File: `internal/storage/lockfile.go`** (new)

Create functions:
- `LoadLockFile() (*models.LockFile, error)` - load from `./agent-lock.json`
- `SaveLockFile(*models.LockFile) error` - save to `./agent-lock.json`
- `AddSkillToLockFile(*models.LockFile, models.LockFileEntry) error`
- `RemoveSkillFromLockFile(*models.LockFile, name string, registry models.Registry) error`
- `GetGitCommit(registryPath string) (string, error)` - get current commit hash

#### Phase 3: Skills Module Updates

**File: `internal/skills/skills.go`**

1. **Modify `DiscoverSkillsInRegistry()`** - Update to not use `registry.Name`
2. **Modify `InstallSkill()`** - Add lock file update after installation
   - Generate unique `installedPath` (handle collisions with `-<sanitized-location>` suffix)
   - Get commit hash for GitHub registries
   - Update lock file
3. **Modify `RemoveSkill()`** - Add lock file update after removal
4. **Add `GenerateInstalledPath(skillName, registryLocation) string`** - Create unique path with collision handling
5. **Add `FormatRegistryDisplay(registry) string`** - Format for display (last 2 components, truncate to 40 chars, colored)

#### Phase 4: Registry TUI Updates

**File: `internal/tui/registry.go`**

1. Remove "add-name" mode from registry flow (registries no longer have names)
2. Update `RegistryItem` to show Type+Location instead of Name
3. Update selection and deletion to use Type+Location as identifier
4. Update help text

#### Phase 5: Skills TUI Updates

**File: `internal/tui/skills.go`**

1. Update `saveSelections()` to:
   - Load existing lock file (if exists)
   - Add new skills to lock file with generated paths
   - Remove unselected managed skills from lock file
   - Only remove filesystem entries for skills in lock file (ignore manually installed)
2. Update skill list display to show registry location (colored) instead of name
3. Handle duplicate skill names across registries in display

#### Phase 6: Storage Updates

**File: `internal/storage/storage.go`**

1. Update `AddRegistry()` - Remove name duplicate check
2. Update `RemoveRegistry()` - Use Type+Location instead of name

### File Changes Summary

| File | Change Type | Description |
|------|-------------|-------------|
| `internal/models/models.go` | Modify | Update Registry and Skill structs, add LockFile types |
| `internal/storage/lockfile.go` | Create | New file for lock file operations |
| `internal/skills/skills.go` | Modify | Update install/remove logic with lock file integration |
| `internal/tui/registry.go` | Modify | Remove name-based flow, use Type+Location |
| `internal/tui/skills.go` | Modify | Integrate lock file, update display |
| `internal/storage/storage.go` | Modify | Remove name-based operations |

### Key Design Decisions

1. **Collision Handling**: Skills installed from different registries with same name get `-<sanitized-location>` suffix where sanitized-location is lowercase, hyphen-separated registry location

2. **Display Format**: Registry location shown as last 2 path components, truncated to 40 chars, colored in TUI

3. **Backward Compatibility**: Skills without lock file entries are treated as manually managed and won't be touched

4. **No Update Command**: Commit hash tracked but no conflict resolution for now

5. **Git Commit Tracking**: Only for GitHub registries, captured at installation time


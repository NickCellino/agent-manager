package models

// RegistryType represents the type of registry (GitHub or local)
type RegistryType string

const (
	RegistryTypeGitHub RegistryType = "github"
	RegistryTypeLocal  RegistryType = "local"
)

// Registry represents a skill registry
type Registry struct {
	Type     RegistryType `json:"type"`
	Location string       `json:"location"` // For GitHub: "owner/repo", for local: absolute path
}

// Skill represents a skill that can be installed
type Skill struct {
	Name       string   `json:"name"`
	Registry   Registry `json:"registry"`
	SourcePath string   `json:"source_path"` // Path within the registry
}

// Agent represents an agent that can be installed
type Agent struct {
	Name       string   `json:"name"`
	Registry   Registry `json:"registry"`
	SourcePath string   `json:"source_path"` // Path to the .md file within the registry
}

// RegistryStore represents the persisted registry configuration
type RegistryStore struct {
	Registries []Registry `json:"registries"`
}

// LockFileEntry represents a single skill entry in the lock file
type LockFileEntry struct {
	Name          string   `json:"name"`
	InstalledPath string   `json:"installedPath"`
	Registry      Registry `json:"registry"`
	Commit        string   `json:"commit,omitempty"` // Only for GitHub registries
}

// LockFile represents the agent-lock.json file structure
type LockFile struct {
	Skills []LockFileEntry `json:"skills"`
	Agents []LockFileEntry `json:"agents"`
}

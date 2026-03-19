package models

// RegistryType represents the type of registry (GitHub or local)
type RegistryType string

const (
	RegistryTypeGitHub RegistryType = "github"
	RegistryTypeLocal  RegistryType = "local"
)

// Registry represents a skill registry
type Registry struct {
	Name     string       `json:"name"`
	Type     RegistryType `json:"type"`
	Location string       `json:"location"` // For GitHub: "owner/repo", for local: absolute path
}

// Skill represents a skill that can be installed
type Skill struct {
	Name         string       `json:"name"`
	Registry     string       `json:"registry"`
	RegistryType RegistryType `json:"registry_type"`
	SourcePath   string       `json:"source_path"` // Path within the registry
}

// RegistryStore represents the persisted registry configuration
type RegistryStore struct {
	Registries []Registry `json:"registries"`
}

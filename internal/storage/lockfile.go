package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"agent-manager/internal/models"
)

// LockFilePath returns the path to the agent-lock.json file in the current project
func LockFilePath() string {
	return "agent-lock.json"
}

// LoadLockFile loads the lock file from the current project directory
func LoadLockFile() (*models.LockFile, error) {
	filePath := LockFilePath()

	// If file doesn't exist, return empty lock file
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return &models.LockFile{
			Skills: []models.LockFileEntry{},
			Agents: []models.LockFileEntry{},
		}, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read lock file: %w", err)
	}

	var lockFile models.LockFile
	if err := json.Unmarshal(data, &lockFile); err != nil {
		return nil, fmt.Errorf("failed to parse lock file: %w", err)
	}

	// Ensure slices are non-nil for consistent behavior
	if lockFile.Skills == nil {
		lockFile.Skills = []models.LockFileEntry{}
	}
	if lockFile.Agents == nil {
		lockFile.Agents = []models.LockFileEntry{}
	}

	return &lockFile, nil
}

// SaveLockFile saves the lock file to the current project directory
func SaveLockFile(lockFile *models.LockFile) error {
	data, err := json.MarshalIndent(lockFile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal lock file: %w", err)
	}

	filePath := LockFilePath()
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write lock file: %w", err)
	}

	return nil
}

// AddSkillToLockFile adds a skill entry to the lock file
// Returns error if skill with same name and registry already exists
func AddSkillToLockFile(lockFile *models.LockFile, entry models.LockFileEntry) error {
	// Check if skill already exists
	for i, existing := range lockFile.Skills {
		if existing.Name == entry.Name && existing.Registry.Type == entry.Registry.Type && existing.Registry.Location == entry.Registry.Location {
			// Update existing entry
			lockFile.Skills[i] = entry
			return SaveLockFile(lockFile)
		}
	}

	// Add new entry
	lockFile.Skills = append(lockFile.Skills, entry)
	return SaveLockFile(lockFile)
}

// RemoveSkillFromLockFile removes a skill entry from the lock file
// Returns error if skill not found
func RemoveSkillFromLockFile(lockFile *models.LockFile, name string, registry models.Registry) error {
	for i, entry := range lockFile.Skills {
		if entry.Name == name && entry.Registry.Type == registry.Type && entry.Registry.Location == registry.Location {
			lockFile.Skills = append(lockFile.Skills[:i], lockFile.Skills[i+1:]...)
			return SaveLockFile(lockFile)
		}
	}
	return fmt.Errorf("skill '%s' from registry '%s' not found in lock file", name, registry.Location)
}

// GetGitCommit gets the current commit hash for a git repository
func GetGitCommit(repoPath string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get git commit: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// SanitizeRegistryLocation converts a registry location to a filesystem-safe string
// e.g., "NickCellino/laptop-setup" -> "nickcellino-laptop-setup"
func SanitizeRegistryLocation(location string) string {
	// Convert to lowercase
	sanitized := strings.ToLower(location)
	// Replace path separators and other special chars with hyphens
	sanitized = regexp.MustCompile(`[/_\\]+`).ReplaceAllString(sanitized, "-")
	// Remove any remaining non-alphanumeric characters except hyphens
	sanitized = regexp.MustCompile(`[^a-z0-9-]`).ReplaceAllString(sanitized, "")
	// Collapse multiple hyphens
	sanitized = regexp.MustCompile(`-+`).ReplaceAllString(sanitized, "-")
	// Trim hyphens from start and end
	sanitized = strings.Trim(sanitized, "-")
	return sanitized
}

// GenerateInstalledPath generates a unique installed path for a skill
// Checks both lock file and filesystem to avoid collisions
func GenerateInstalledPath(skillName string, registry models.Registry, lockFile *models.LockFile, skillsDir string) string {
	// First, check if this exact skill (same name + registry) is already in the lock file
	for _, entry := range lockFile.Skills {
		if entry.Name == skillName {
			if entry.Registry.Type == registry.Type && entry.Registry.Location == registry.Location {
				// Same skill from same registry - use existing path
				return entry.InstalledPath
			}
			// Same skill name but different registry - need to differentiate
			sanitized := SanitizeRegistryLocation(registry.Location)
			return fmt.Sprintf("%s-%s", skillName, sanitized)
		}
	}

	// Check if something already exists at the target path (manually installed or unmanaged)
	targetPath := skillName
	fullPath := fmt.Sprintf("%s/%s", skillsDir, targetPath)
	if _, err := os.Stat(fullPath); err == nil {
		// Path already exists - append registry location to differentiate
		sanitized := SanitizeRegistryLocation(registry.Location)
		return fmt.Sprintf("%s-%s", skillName, sanitized)
	}

	// No collision, use skill name as-is
	return skillName
}

// FindLockFileEntry finds a lock file entry by skill name and registry
func FindLockFileEntry(lockFile *models.LockFile, name string, registry models.Registry) *models.LockFileEntry {
	for _, entry := range lockFile.Skills {
		if entry.Name == name && entry.Registry.Type == registry.Type && entry.Registry.Location == registry.Location {
			return &entry
		}
	}
	return nil
}

// IsManagedSkill checks if a skill is managed by agent-manager (exists in lock file)
func IsManagedSkill(lockFile *models.LockFile, skillName string) bool {
	for _, entry := range lockFile.Skills {
		if entry.InstalledPath == skillName {
			return true
		}
	}
	return false
}

// GetManagedSkillEntry gets the lock file entry for an installed skill by its path name
func GetManagedSkillEntry(lockFile *models.LockFile, installedPath string) *models.LockFileEntry {
	for _, entry := range lockFile.Skills {
		if entry.InstalledPath == installedPath {
			return &entry
		}
	}
	return nil
}

// AddAgentToLockFile adds an agent entry to the lock file.
// If an entry with the same name and registry already exists it is updated.
func AddAgentToLockFile(lockFile *models.LockFile, entry models.LockFileEntry) error {
	for i, existing := range lockFile.Agents {
		if existing.Name == entry.Name && existing.Registry.Type == entry.Registry.Type && existing.Registry.Location == entry.Registry.Location {
			lockFile.Agents[i] = entry
			return SaveLockFile(lockFile)
		}
	}
	lockFile.Agents = append(lockFile.Agents, entry)
	return SaveLockFile(lockFile)
}

// RemoveAgentFromLockFile removes an agent entry from the lock file.
func RemoveAgentFromLockFile(lockFile *models.LockFile, name string, registry models.Registry) error {
	for i, entry := range lockFile.Agents {
		if entry.Name == name && entry.Registry.Type == registry.Type && entry.Registry.Location == registry.Location {
			lockFile.Agents = append(lockFile.Agents[:i], lockFile.Agents[i+1:]...)
			return SaveLockFile(lockFile)
		}
	}
	return fmt.Errorf("agent '%s' from registry '%s' not found in lock file", name, registry.Location)
}

// FindAgentLockFileEntry finds an agent lock file entry by name and registry.
func FindAgentLockFileEntry(lockFile *models.LockFile, name string, registry models.Registry) *models.LockFileEntry {
	for i, entry := range lockFile.Agents {
		if entry.Name == name && entry.Registry.Type == registry.Type && entry.Registry.Location == registry.Location {
			return &lockFile.Agents[i]
		}
	}
	return nil
}

// IsManagedAgent checks if an agent is managed by agent-manager (exists in lock file)
func IsManagedAgent(lockFile *models.LockFile, agentName string) bool {
	for _, entry := range lockFile.Agents {
		if entry.InstalledPath == agentName {
			return true
		}
	}
	return false
}

// GetManagedAgentEntry gets the lock file entry for an installed agent by its path name
func GetManagedAgentEntry(lockFile *models.LockFile, installedPath string) *models.LockFileEntry {
	for i, entry := range lockFile.Agents {
		if entry.InstalledPath == installedPath {
			return &lockFile.Agents[i]
		}
	}
	return nil
}

// GenerateInstalledAgentPath generates a unique installed path for an agent.
// Similar to GenerateInstalledPath but checks for .md files on the filesystem.
func GenerateInstalledAgentPath(agentName string, registry models.Registry, lockFile *models.LockFile, agentsDir string) string {
	// Check if this exact agent (same name + registry) is already in the lock file
	for _, entry := range lockFile.Agents {
		if entry.Name == agentName {
			if entry.Registry.Type == registry.Type && entry.Registry.Location == registry.Location {
				return entry.InstalledPath
			}
			// Same agent name but different registry - need to differentiate
			sanitized := SanitizeRegistryLocation(registry.Location)
			return fmt.Sprintf("%s-%s", agentName, sanitized)
		}
	}

	// Check if a file already exists at the target path (manually installed or unmanaged)
	fullPath := fmt.Sprintf("%s/%s.md", agentsDir, agentName)
	if _, err := os.Stat(fullPath); err == nil {
		sanitized := SanitizeRegistryLocation(registry.Location)
		return fmt.Sprintf("%s-%s", agentName, sanitized)
	}

	return agentName
}

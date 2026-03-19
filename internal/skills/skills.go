package skills

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"agent-manager/internal/models"
	"agent-manager/internal/storage"
)

// DiscoverSkills finds all skills in all registries
func DiscoverSkills() ([]models.Skill, error) {
	store, err := storage.LoadRegistries()
	if err != nil {
		return nil, err
	}

	var allSkills []models.Skill

	for _, registry := range store.Registries {
		skills, err := DiscoverSkillsInRegistry(registry)
		if err != nil {
			// Log error but continue with other registries
			fmt.Fprintf(os.Stderr, "Warning: failed to discover skills in registry %s: %v\n", registry.Location, err)
			continue
		}
		allSkills = append(allSkills, skills...)
	}

	return allSkills, nil
}

// DiscoverSkillsInRegistry finds all skills in a specific registry
func DiscoverSkillsInRegistry(registry models.Registry) ([]models.Skill, error) {
	var registryPath string

	switch registry.Type {
	case models.RegistryTypeGitHub:
		registryPath = getGitHubRegistryPath(registry.Location)
		// Ensure the registry is cloned
		if _, err := os.Stat(registryPath); os.IsNotExist(err) {
			if err := CloneGitHubRegistry(registry.Location); err != nil {
				return nil, err
			}
		}
	case models.RegistryTypeLocal:
		// Expand ~ to home directory if needed
		if strings.HasPrefix(registry.Location, "~/") {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, err
			}
			registryPath = filepath.Join(home, registry.Location[2:])
		} else {
			registryPath = registry.Location
		}
	default:
		return nil, fmt.Errorf("unknown registry type: %s", registry.Type)
	}

	// Check if registry path exists
	if _, err := os.Stat(registryPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("registry path does not exist: %s", registryPath)
	}

	var skills []models.Skill

	// Search in .agents/skills
	agentsSkillsPath := filepath.Join(registryPath, ".agents", "skills")
	if agentsSkills, err := listSkillsInDir(agentsSkillsPath, registry); err == nil {
		skills = append(skills, agentsSkills...)
	}

	// Search in .opencode/skills
	opencodeSkillsPath := filepath.Join(registryPath, ".opencode", "skills")
	if opencodeSkills, err := listSkillsInDir(opencodeSkillsPath, registry); err == nil {
		skills = append(skills, opencodeSkills...)
	}

	return skills, nil
}

// listSkillsInDir lists all skills in a directory
func listSkillsInDir(dir string, registry models.Registry) ([]models.Skill, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var skills []models.Skill
	for _, entry := range entries {
		if entry.IsDir() {
			skills = append(skills, models.Skill{
				Name:       entry.Name(),
				Registry:   registry,
				SourcePath: filepath.Join(dir, entry.Name()),
			})
		}
	}

	return skills, nil
}

// getGitHubRegistryPath returns the local path for a GitHub registry
func getGitHubRegistryPath(location string) string {
	return filepath.Join(storage.GitHubRegistriesDir(), location)
}

// CloneGitHubRegistry clones a GitHub repository to the local cache
func CloneGitHubRegistry(location string) error {
	repoURL := fmt.Sprintf("https://github.com/%s.git", location)
	targetPath := getGitHubRegistryPath(location)

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Clone the repository
	cmd := exec.Command("git", "clone", repoURL, targetPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to clone repository: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// InstallSkill installs a skill to the target directory
// The targetName parameter allows installing with a different name (e.g., for collision handling)
func InstallSkill(skill models.Skill, targetDir string, targetName string) error {
	if targetName == "" {
		targetName = skill.Name
	}

	targetPath := filepath.Join(targetDir, targetName)

	// Check if skill already exists (use Lstat to not follow symlinks)
	if _, err := os.Lstat(targetPath); err == nil {
		return fmt.Errorf("skill already exists at %s", targetPath)
	}

	switch skill.Registry.Type {
	case models.RegistryTypeGitHub:
		// Copy the skill from the cloned repository
		return copyDir(skill.SourcePath, targetPath)
	case models.RegistryTypeLocal:
		// Create symlink for local registries
		return os.Symlink(skill.SourcePath, targetPath)
	default:
		return fmt.Errorf("unknown registry type: %s", skill.Registry.Type)
	}
}

// RemoveSkill removes a skill from the target directory
func RemoveSkill(skillName string, targetDir string) error {
	targetPath := filepath.Join(targetDir, skillName)

	// Check if it's a symlink
	info, err := os.Lstat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Already removed
		}
		return err
	}

	if info.Mode()&os.ModeSymlink != 0 {
		// It's a symlink, just remove it
		return os.Remove(targetPath)
	}

	// It's a regular directory, remove it
	return os.RemoveAll(targetPath)
}

// copyDir copies a directory recursively
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		// Copy file
		return copyFile(path, targetPath)
	})
}

// copyFile copies a file
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = destFile.ReadFrom(sourceFile)
	if err != nil {
		return err
	}

	// Copy file permissions
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, info.Mode())
}

// GetProjectSkillsDir returns the path to the project's skills directory
func GetProjectSkillsDir() string {
	// First check for .opencode/skills
	if _, err := os.Stat(".opencode"); err == nil {
		return ".opencode/skills"
	}
	// Default to .opencode/skills (will be created if needed)
	return ".opencode/skills"
}

// EnsureProjectSkillsDir ensures the project's skills directory exists
func EnsureProjectSkillsDir() (string, error) {
	skillsDir := GetProjectSkillsDir()
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		return "", err
	}
	return skillsDir, nil
}

// ListInstalledSkills lists all skills installed in the project
func ListInstalledSkills() ([]string, error) {
	skillsDir := GetProjectSkillsDir()

	if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
		return []string{}, nil
	}

	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil, err
	}

	var skills []string
	for _, entry := range entries {
		// Check if it's a directory OR a symlink (symlinks to directories count too)
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			skills = append(skills, entry.Name())
		}
	}

	return skills, nil
}

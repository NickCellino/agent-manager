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

	// Recursively find all directories named "skills"
	filepath.Walk(registryPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip directories we can't access
		}
		if info.IsDir() && info.Name() == "skills" {
			if foundSkills, err := listSkillsInDir(path, registry); err == nil {
				skills = append(skills, foundSkills...)
			}
		}
		return nil
	})

	// Search in Claude-style skills/<name>/SKILL.md registries.
	claudeSkillsPath := filepath.Join(registryPath, "skills")
	if claudeSkills, err := listClaudeSkillsInDir(claudeSkillsPath, registry); err == nil {
		skills = append(skills, claudeSkills...)
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

// listClaudeSkillsInDir lists skills stored as skills/<name>/SKILL.md.
func listClaudeSkillsInDir(dir string, registry models.Registry) ([]models.Skill, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var skills []models.Skill
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillPath := filepath.Join(dir, entry.Name())
		manifestPath := filepath.Join(skillPath, "SKILL.md")
		if _, err := os.Stat(manifestPath); err != nil {
			continue
		}

		skills = append(skills, models.Skill{
			Name:       entry.Name(),
			Registry:   registry,
			SourcePath: skillPath,
		})
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

// AddSkillToProject installs a skill into the current project and records it in
// the lock file. It handles path-collision detection and git commit tracking for
// GitHub registries. The caller is responsible for checking whether the skill is
// already present in the lock file before calling this function.
// Returns the path name the skill was installed under.
func AddSkillToProject(skill models.Skill, lockFile *models.LockFile) (string, error) {
	skillsDir, err := EnsureProjectSkillsDir()
	if err != nil {
		return "", err
	}

	installedPath := storage.GenerateInstalledPath(skill.Name, skill.Registry, lockFile, skillsDir)

	if err := InstallSkill(skill, skillsDir, installedPath); err != nil {
		return "", fmt.Errorf("failed to install skill %s: %w", skill.Name, err)
	}

	var commit string
	if skill.Registry.Type == models.RegistryTypeGitHub {
		commit, _ = storage.GetGitCommit(getGitHubRegistryPath(skill.Registry.Location))
	}

	entry := models.LockFileEntry{
		Name:          skill.Name,
		InstalledPath: installedPath,
		Registry:      skill.Registry,
		Commit:        commit,
	}
	if err := storage.AddSkillToLockFile(lockFile, entry); err != nil {
		return "", fmt.Errorf("failed to update lock file for skill %s: %w", skill.Name, err)
	}

	return installedPath, nil
}

// RemoveSkillFromProject removes a skill from the project filesystem and lock file.
func RemoveSkillFromProject(entry *models.LockFileEntry, lockFile *models.LockFile) error {
	skillsDir := GetProjectSkillsDir()
	if err := RemoveSkill(entry.InstalledPath, skillsDir); err != nil {
		return fmt.Errorf("failed to remove skill %q: %w", entry.Name, err)
	}
	return storage.RemoveSkillFromLockFile(lockFile, entry.Name, entry.Registry)
}

// UpdateSkillInProject updates a skill installed from a GitHub registry to the latest
// version. It pulls the registry, re-copies the skill files, and updates the commit
// hash in the lock file. The entry must be a pointer to an element of lockFile.Skills
// so that in-place updates are reflected when SaveLockFile is called.
func UpdateSkillInProject(entry *models.LockFileEntry, lockFile *models.LockFile) error {
	if entry.Registry.Type != models.RegistryTypeGitHub {
		return fmt.Errorf("skill %q is not from a GitHub registry", entry.Name)
	}

	registryPath := getGitHubRegistryPath(entry.Registry.Location)

	if err := storage.PullGitHubRegistry(registryPath); err != nil {
		return fmt.Errorf("failed to update registry %s: %w", entry.Registry.Location, err)
	}

	// Re-discover the skill in the refreshed registry
	allSkills, err := DiscoverSkillsInRegistry(entry.Registry)
	if err != nil {
		return fmt.Errorf("failed to discover skills in registry: %w", err)
	}

	var found *models.Skill
	for i := range allSkills {
		if allSkills[i].Name == entry.Name {
			found = &allSkills[i]
			break
		}
	}
	if found == nil {
		return fmt.Errorf("skill %q not found in registry %s after update", entry.Name, entry.Registry.Location)
	}

	// Remove old files and re-copy from the updated registry
	skillsDir := GetProjectSkillsDir()
	if err := RemoveSkill(entry.InstalledPath, skillsDir); err != nil {
		return fmt.Errorf("failed to remove old skill files: %w", err)
	}
	if err := InstallSkill(*found, skillsDir, entry.InstalledPath); err != nil {
		return fmt.Errorf("failed to install updated skill: %w", err)
	}

	// Update the commit hash in-place and persist the lock file
	commit, err := storage.GetGitCommit(registryPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to get commit hash for %s: %v\n", entry.Registry.Location, err)
	}
	entry.Commit = commit
	return storage.SaveLockFile(lockFile)
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

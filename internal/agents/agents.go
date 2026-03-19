package agents

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"agent-manager/internal/models"
	"agent-manager/internal/storage"
)

// DiscoverAgents finds all agents across all configured registries.
func DiscoverAgents() ([]models.Agent, error) {
	store, err := storage.LoadRegistries()
	if err != nil {
		return nil, err
	}

	var allAgents []models.Agent
	for _, registry := range store.Registries {
		found, err := DiscoverAgentsInRegistry(registry)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to discover agents in registry %s: %v\n", registry.Location, err)
			continue
		}
		allAgents = append(allAgents, found...)
	}

	return allAgents, nil
}

// DiscoverAgentsInRegistry finds all agents in a specific registry.
// Agents are .md files located under <registry>/.opencode/agents/.
func DiscoverAgentsInRegistry(registry models.Registry) ([]models.Agent, error) {
	var registryPath string

	switch registry.Type {
	case models.RegistryTypeGitHub:
		registryPath = getGitHubRegistryPath(registry.Location)
		if _, err := os.Stat(registryPath); os.IsNotExist(err) {
			if err := cloneGitHubRegistry(registry.Location); err != nil {
				return nil, err
			}
		}
	case models.RegistryTypeLocal:
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

	if _, err := os.Stat(registryPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("registry path does not exist: %s", registryPath)
	}

	agentsPath := filepath.Join(registryPath, ".opencode", "agents")
	if _, err := os.Stat(agentsPath); os.IsNotExist(err) {
		agentsPath = filepath.Join(registryPath, ".opencode", "agent")
	}
	return listAgentsInDir(agentsPath, registry)
}

// listAgentsInDir lists all agent .md files in a directory, recursing into subdirectories.
func listAgentsInDir(dir string, registry models.Registry) ([]models.Agent, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var found []models.Agent
	for _, entry := range entries {
		if entry.IsDir() {
			sub, err := listAgentsInDir(filepath.Join(dir, entry.Name()), registry)
			if err != nil {
				return nil, err
			}
			found = append(found, sub...)
		} else if strings.HasSuffix(entry.Name(), ".md") {
			name := strings.TrimSuffix(entry.Name(), ".md")
			found = append(found, models.Agent{
				Name:       name,
				Registry:   registry,
				SourcePath: filepath.Join(dir, entry.Name()),
			})
		}
	}

	return found, nil
}

// getGitHubRegistryPath returns the local cache path for a GitHub registry.
func getGitHubRegistryPath(location string) string {
	return filepath.Join(storage.GitHubRegistriesDir(), location)
}

// cloneGitHubRegistry clones a GitHub repository to the local cache.
func cloneGitHubRegistry(location string) error {
	repoURL := fmt.Sprintf("https://github.com/%s.git", location)
	targetPath := getGitHubRegistryPath(location)

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	cmd := exec.Command("git", "clone", repoURL, targetPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to clone repository: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// InstallAgent installs an agent .md file into the target directory.
// For GitHub registries the file is copied; for local registries a symlink is created.
func InstallAgent(agent models.Agent, targetDir string, targetName string) error {
	if targetName == "" {
		targetName = agent.Name
	}

	targetPath := filepath.Join(targetDir, targetName+".md")

	if _, err := os.Lstat(targetPath); err == nil {
		return fmt.Errorf("agent already exists at %s", targetPath)
	}

	switch agent.Registry.Type {
	case models.RegistryTypeGitHub:
		return copyFile(agent.SourcePath, targetPath)
	case models.RegistryTypeLocal:
		return os.Symlink(agent.SourcePath, targetPath)
	default:
		return fmt.Errorf("unknown registry type: %s", agent.Registry.Type)
	}
}

// RemoveAgent removes an agent .md file from the target directory.
func RemoveAgent(agentName string, targetDir string) error {
	targetPath := filepath.Join(targetDir, agentName+".md")

	if _, err := os.Lstat(targetPath); err != nil {
		if os.IsNotExist(err) {
			return nil // Already removed
		}
		return err
	}

	return os.Remove(targetPath)
}

// copyFile copies a single file from src to dst.
func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	if _, err = destFile.ReadFrom(sourceFile); err != nil {
		return err
	}

	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, info.Mode())
}

// AddAgentToProject installs an agent into the current project and records it in
// the lock file. The caller is responsible for checking whether the agent is
// already present in the lock file before calling this function.
// Returns the installed path name.
func AddAgentToProject(agent models.Agent, lockFile *models.LockFile) (string, error) {
	agentsDir, err := EnsureProjectAgentsDir()
	if err != nil {
		return "", err
	}

	installedPath := storage.GenerateInstalledAgentPath(agent.Name, agent.Registry, lockFile, agentsDir)

	if err := InstallAgent(agent, agentsDir, installedPath); err != nil {
		return "", fmt.Errorf("failed to install agent %s: %w", agent.Name, err)
	}

	var commit string
	if agent.Registry.Type == models.RegistryTypeGitHub {
		commit, _ = storage.GetGitCommit(getGitHubRegistryPath(agent.Registry.Location))
	}

	entry := models.LockFileEntry{
		Name:          agent.Name,
		InstalledPath: installedPath,
		Registry:      agent.Registry,
		Commit:        commit,
	}
	if err := storage.AddAgentToLockFile(lockFile, entry); err != nil {
		return "", fmt.Errorf("failed to update lock file for agent %s: %w", agent.Name, err)
	}

	return installedPath, nil
}

// RemoveAgentFromProject removes an agent from the project filesystem and lock file.
func RemoveAgentFromProject(entry *models.LockFileEntry, lockFile *models.LockFile) error {
	agentsDir := GetProjectAgentsDir()
	if err := RemoveAgent(entry.InstalledPath, agentsDir); err != nil {
		return fmt.Errorf("failed to remove agent %q: %w", entry.Name, err)
	}
	return storage.RemoveAgentFromLockFile(lockFile, entry.Name, entry.Registry)
}

// GetProjectAgentsDir returns the path to the project's agents directory.
func GetProjectAgentsDir() string {
	return ".opencode/agents"
}

// EnsureProjectAgentsDir ensures the project's agents directory exists.
func EnsureProjectAgentsDir() (string, error) {
	agentsDir := GetProjectAgentsDir()
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		return "", err
	}
	return agentsDir, nil
}

// ListInstalledAgents lists all agent names installed in the project.
func ListInstalledAgents() ([]string, error) {
	agentsDir := GetProjectAgentsDir()

	if _, err := os.Stat(agentsDir); os.IsNotExist(err) {
		return []string{}, nil
	}

	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		return nil, err
	}

	var found []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			found = append(found, strings.TrimSuffix(entry.Name(), ".md"))
		}
	}

	return found, nil
}

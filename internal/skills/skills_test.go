package skills_test

import (
	"os"
	"path/filepath"
	"testing"

	"agent-manager/internal/models"
	"agent-manager/internal/skills"
	"agent-manager/internal/storage"
)

func TestSkillDiscoveryAndInstallation(t *testing.T) {
	// Use isolated XDG_DATA_HOME
	testDataDir := t.TempDir()
	os.Setenv("XDG_DATA_HOME", testDataDir)

	// Create test registry structure
	testRegistryDir := t.TempDir()
	skillsDir := filepath.Join(testRegistryDir, ".opencode", "skills", "test-skill")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("Failed to create test skill dir: %v", err)
	}

	// Create a skill file
	skillFile := filepath.Join(skillsDir, "skill.yaml")
	if err := os.WriteFile(skillFile, []byte("name: test-skill\n"), 0644); err != nil {
		t.Fatalf("Failed to create skill file: %v", err)
	}

	// Create registry config
	store := &models.RegistryStore{
		Registries: []models.Registry{
			{
				Type:     models.RegistryTypeLocal,
				Location: testRegistryDir,
			},
		},
	}

	if err := storage.SaveRegistries(store); err != nil {
		t.Fatalf("Failed to save registries: %v", err)
	}

	// Test 1: Discover skills
	allSkills, err := skills.DiscoverSkills()
	if err != nil {
		t.Fatalf("Failed to discover skills: %v", err)
	}

	if len(allSkills) != 1 {
		t.Fatalf("Expected 1 skill, got %d", len(allSkills))
	}

	if allSkills[0].Name != "test-skill" {
		t.Fatalf("Expected skill name 'test-skill', got '%s'", allSkills[0].Name)
	}

	// Test 2: Install skill
	targetDir := t.TempDir()
	if err := skills.InstallSkill(allSkills[0], targetDir, ""); err != nil {
		t.Fatalf("Failed to install skill: %v", err)
	}

	// Verify installation
	installedSkillPath := filepath.Join(targetDir, "test-skill")
	if _, err := os.Stat(installedSkillPath); os.IsNotExist(err) {
		t.Fatalf("Skill was not installed at %s", installedSkillPath)
	}

	// Verify it's a symlink for local registries
	info, err := os.Lstat(installedSkillPath)
	if err != nil {
		t.Fatalf("Failed to stat installed skill: %v", err)
	}

	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("Expected symlink for local registry skill")
	}

	// Test 3: List installed skills
	// Note: ListInstalledSkills looks in current directory's .opencode/skills
	// So we need to change to a directory with the installed skill
	cwd, _ := os.Getwd()
	testProjectDir := t.TempDir()
	projectSkillsDir := filepath.Join(testProjectDir, ".opencode", "skills")
	if err := os.MkdirAll(projectSkillsDir, 0755); err != nil {
		t.Fatalf("Failed to create project skills dir: %v", err)
	}

	// Create a test skill in the project
	testSkillDir := filepath.Join(projectSkillsDir, "project-skill")
	if err := os.MkdirAll(testSkillDir, 0755); err != nil {
		t.Fatalf("Failed to create test skill in project: %v", err)
	}
	if err := os.WriteFile(filepath.Join(testSkillDir, "skill.yaml"), []byte("name: project-skill\n"), 0644); err != nil {
		t.Fatalf("Failed to create test skill file: %v", err)
	}

	os.Chdir(testProjectDir)
	defer os.Chdir(cwd)

	installed, err := skills.ListInstalledSkills()
	if err != nil {
		t.Fatalf("Failed to list installed skills: %v", err)
	}

	if len(installed) != 1 || installed[0] != "project-skill" {
		t.Fatalf("Expected [project-skill], got %v", installed)
	}
}

func TestClaudeStyleSkillDiscovery(t *testing.T) {
	registryDir := t.TempDir()
	skillDir := filepath.Join(registryDir, "skills", "agent-browser")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create Claude-style skill dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\ndescription: Browser automation\n---\n"), 0644); err != nil {
		t.Fatalf("Failed to create SKILL.md: %v", err)
	}

	registry := models.Registry{
		Type:     models.RegistryTypeLocal,
		Location: registryDir,
	}

	discovered, err := skills.DiscoverSkillsInRegistry(registry)
	if err != nil {
		t.Fatalf("Failed to discover Claude-style skills: %v", err)
	}

	if len(discovered) != 1 {
		t.Fatalf("Expected 1 Claude-style skill, got %d", len(discovered))
	}

	if discovered[0].Name != "agent-browser" {
		t.Fatalf("Expected skill name 'agent-browser', got '%s'", discovered[0].Name)
	}

	if discovered[0].SourcePath != skillDir {
		t.Fatalf("Expected source path %q, got %q", skillDir, discovered[0].SourcePath)
	}
}

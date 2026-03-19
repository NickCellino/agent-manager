package storage_test

import (
	"os"
	"testing"

	"agent-manager/internal/models"
	"agent-manager/internal/storage"
)

func setupStorageTest(t *testing.T) {
	t.Helper()
	testDataDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", testDataDir)
}

func TestAddRegistry(t *testing.T) {
	setupStorageTest(t)

	store := &models.RegistryStore{}

	reg := models.Registry{Type: models.RegistryTypeLocal, Location: "/tmp/skills"}
	if err := storage.AddRegistry(store, reg); err != nil {
		t.Fatalf("AddRegistry failed: %v", err)
	}

	if len(store.Registries) != 1 {
		t.Fatalf("Expected 1 registry, got %d", len(store.Registries))
	}

	// Adding a duplicate should error
	if err := storage.AddRegistry(store, reg); err == nil {
		t.Fatal("Expected error when adding duplicate registry, got nil")
	}
}

func TestRemoveRegistry(t *testing.T) {
	setupStorageTest(t)

	store := &models.RegistryStore{}
	reg := models.Registry{Type: models.RegistryTypeLocal, Location: "/tmp/skills"}
	if err := storage.AddRegistry(store, reg); err != nil {
		t.Fatalf("AddRegistry failed: %v", err)
	}

	if err := storage.RemoveRegistry(store, reg.Type, reg.Location); err != nil {
		t.Fatalf("RemoveRegistry failed: %v", err)
	}
	if len(store.Registries) != 0 {
		t.Fatalf("Expected 0 registries after removal, got %d", len(store.Registries))
	}

	// Removing a non-existent registry should error
	if err := storage.RemoveRegistry(store, reg.Type, reg.Location); err == nil {
		t.Fatal("Expected error when removing non-existent registry, got nil")
	}
}

func TestLoadSaveRegistries(t *testing.T) {
	setupStorageTest(t)

	store := &models.RegistryStore{
		Registries: []models.Registry{
			{Type: models.RegistryTypeGitHub, Location: "owner/repo"},
			{Type: models.RegistryTypeLocal, Location: "/tmp/local"},
		},
	}

	if err := storage.SaveRegistries(store); err != nil {
		t.Fatalf("SaveRegistries failed: %v", err)
	}

	loaded, err := storage.LoadRegistries()
	if err != nil {
		t.Fatalf("LoadRegistries failed: %v", err)
	}

	if len(loaded.Registries) != 2 {
		t.Fatalf("Expected 2 registries, got %d", len(loaded.Registries))
	}
	if loaded.Registries[0].Location != "owner/repo" {
		t.Errorf("Expected 'owner/repo', got '%s'", loaded.Registries[0].Location)
	}
}

func TestLoadRegistries_Empty(t *testing.T) {
	setupStorageTest(t)

	// No file written yet – should return an empty store
	loaded, err := storage.LoadRegistries()
	if err != nil {
		t.Fatalf("LoadRegistries failed: %v", err)
	}
	if len(loaded.Registries) != 0 {
		t.Fatalf("Expected 0 registries, got %d", len(loaded.Registries))
	}
}

func TestSanitizeRegistryLocation(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"NickCellino/laptop-setup", "nickcellino-laptop-setup"},
		{"owner/repo", "owner-repo"},
		{"/home/user/my skills/", "home-user-myskills"},
		{"UPPER/CASE", "upper-case"},
		{"multi//slash", "multi-slash"},
		{"/leading-slash", "leading-slash"},
		{"trailing-slash/", "trailing-slash"},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := storage.SanitizeRegistryLocation(tc.input)
			if got != tc.expected {
				t.Errorf("SanitizeRegistryLocation(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestGenerateInstalledPath_NoCollision(t *testing.T) {
	setupStorageTest(t)

	skillsDir := t.TempDir()
	lockFile := &models.LockFile{Skills: []models.LockFileEntry{}}
	reg := models.Registry{Type: models.RegistryTypeLocal, Location: "/tmp/reg"}

	path := storage.GenerateInstalledPath("myskill", reg, lockFile, skillsDir)
	if path != "myskill" {
		t.Errorf("Expected 'myskill', got %q", path)
	}
}

func TestGenerateInstalledPath_FilesystemCollision(t *testing.T) {
	setupStorageTest(t)

	skillsDir := t.TempDir()
	// Create a pre-existing directory to simulate an unmanaged skill
	if err := os.MkdirAll(skillsDir+"/myskill", 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}

	lockFile := &models.LockFile{Skills: []models.LockFileEntry{}}
	reg := models.Registry{Type: models.RegistryTypeLocal, Location: "/tmp/reg"}

	path := storage.GenerateInstalledPath("myskill", reg, lockFile, skillsDir)
	if path == "myskill" {
		t.Errorf("Expected a path with registry suffix due to collision, got %q", path)
	}
}

func TestGenerateInstalledPath_LockFileCollision(t *testing.T) {
	setupStorageTest(t)

	skillsDir := t.TempDir()
	reg1 := models.Registry{Type: models.RegistryTypeLocal, Location: "/tmp/reg1"}
	reg2 := models.Registry{Type: models.RegistryTypeLocal, Location: "/tmp/reg2"}

	lockFile := &models.LockFile{
		Skills: []models.LockFileEntry{
			{Name: "myskill", InstalledPath: "myskill", Registry: reg1},
		},
	}

	// Same skill name, different registry – should get a suffixed path
	path := storage.GenerateInstalledPath("myskill", reg2, lockFile, skillsDir)
	if path == "myskill" {
		t.Errorf("Expected a path with registry suffix for lock-file collision, got %q", path)
	}
}

func TestGenerateInstalledPath_SameRegistryReturnsExistingPath(t *testing.T) {
	setupStorageTest(t)

	skillsDir := t.TempDir()
	reg := models.Registry{Type: models.RegistryTypeLocal, Location: "/tmp/reg"}

	lockFile := &models.LockFile{
		Skills: []models.LockFileEntry{
			{Name: "myskill", InstalledPath: "myskill-custom", Registry: reg},
		},
	}

	// Same name + same registry → return the existing installedPath
	path := storage.GenerateInstalledPath("myskill", reg, lockFile, skillsDir)
	if path != "myskill-custom" {
		t.Errorf("Expected 'myskill-custom', got %q", path)
	}
}

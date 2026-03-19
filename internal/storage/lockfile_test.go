package storage_test

import (
	"os"
	"path/filepath"
	"testing"

	"agent-manager/internal/models"
	"agent-manager/internal/storage"
)

func setupLockFileTest(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// Change to temp dir so LoadLockFile / SaveLockFile operate there
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
	return dir
}

func TestLoadLockFile_Empty(t *testing.T) {
	setupLockFileTest(t)

	lf, err := storage.LoadLockFile()
	if err != nil {
		t.Fatalf("LoadLockFile failed: %v", err)
	}
	if len(lf.Skills) != 0 {
		t.Fatalf("Expected 0 skills, got %d", len(lf.Skills))
	}
}

func TestSaveLockFile(t *testing.T) {
	dir := setupLockFileTest(t)

	lf := &models.LockFile{
		Skills: []models.LockFileEntry{
			{Name: "myskill", InstalledPath: "myskill", Registry: models.Registry{Type: models.RegistryTypeLocal, Location: "/tmp/reg"}},
		},
	}

	if err := storage.SaveLockFile(lf); err != nil {
		t.Fatalf("SaveLockFile failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "agent-lock.json")); os.IsNotExist(err) {
		t.Fatal("agent-lock.json was not created")
	}

	loaded, err := storage.LoadLockFile()
	if err != nil {
		t.Fatalf("LoadLockFile after save failed: %v", err)
	}
	if len(loaded.Skills) != 1 || loaded.Skills[0].Name != "myskill" {
		t.Fatalf("Unexpected lock file contents: %+v", loaded.Skills)
	}
}

func TestAddSkillToLockFile(t *testing.T) {
	setupLockFileTest(t)

	lf := &models.LockFile{Skills: []models.LockFileEntry{}}
	reg := models.Registry{Type: models.RegistryTypeLocal, Location: "/tmp/reg"}
	entry := models.LockFileEntry{Name: "myskill", InstalledPath: "myskill", Registry: reg}

	if err := storage.AddSkillToLockFile(lf, entry); err != nil {
		t.Fatalf("AddSkillToLockFile failed: %v", err)
	}
	if len(lf.Skills) != 1 {
		t.Fatalf("Expected 1 skill, got %d", len(lf.Skills))
	}

	// Adding again (same name + registry) should update, not duplicate
	updated := entry
	updated.Commit = "abc123"
	if err := storage.AddSkillToLockFile(lf, updated); err != nil {
		t.Fatalf("AddSkillToLockFile update failed: %v", err)
	}
	if len(lf.Skills) != 1 {
		t.Fatalf("Expected still 1 skill after update, got %d", len(lf.Skills))
	}
	if lf.Skills[0].Commit != "abc123" {
		t.Errorf("Expected commit 'abc123', got %q", lf.Skills[0].Commit)
	}
}

func TestRemoveSkillFromLockFile(t *testing.T) {
	setupLockFileTest(t)

	reg := models.Registry{Type: models.RegistryTypeLocal, Location: "/tmp/reg"}
	lf := &models.LockFile{
		Skills: []models.LockFileEntry{
			{Name: "myskill", InstalledPath: "myskill", Registry: reg},
		},
	}

	if err := storage.RemoveSkillFromLockFile(lf, "myskill", reg); err != nil {
		t.Fatalf("RemoveSkillFromLockFile failed: %v", err)
	}
	if len(lf.Skills) != 0 {
		t.Fatalf("Expected 0 skills after removal, got %d", len(lf.Skills))
	}

	// Removing again should error
	if err := storage.RemoveSkillFromLockFile(lf, "myskill", reg); err == nil {
		t.Fatal("Expected error when removing non-existent skill, got nil")
	}
}

func TestIsManagedSkill(t *testing.T) {
	reg := models.Registry{Type: models.RegistryTypeLocal, Location: "/tmp/reg"}
	lf := &models.LockFile{
		Skills: []models.LockFileEntry{
			{Name: "myskill", InstalledPath: "myskill-installed", Registry: reg},
		},
	}

	if !storage.IsManagedSkill(lf, "myskill-installed") {
		t.Error("Expected 'myskill-installed' to be managed")
	}
	if storage.IsManagedSkill(lf, "myskill") {
		t.Error("Expected 'myskill' (skill name, not installedPath) to NOT be managed")
	}
	if storage.IsManagedSkill(lf, "unknown") {
		t.Error("Expected 'unknown' to not be managed")
	}
}

func TestFindLockFileEntry(t *testing.T) {
	reg1 := models.Registry{Type: models.RegistryTypeLocal, Location: "/tmp/reg1"}
	reg2 := models.Registry{Type: models.RegistryTypeGitHub, Location: "owner/repo"}
	lf := &models.LockFile{
		Skills: []models.LockFileEntry{
			{Name: "skill-a", InstalledPath: "skill-a", Registry: reg1},
			{Name: "skill-a", InstalledPath: "skill-a-owner-repo", Registry: reg2},
		},
	}

	found := storage.FindLockFileEntry(lf, "skill-a", reg1)
	if found == nil {
		t.Fatal("Expected to find skill-a from reg1")
	}
	if found.InstalledPath != "skill-a" {
		t.Errorf("Expected installedPath 'skill-a', got %q", found.InstalledPath)
	}

	found2 := storage.FindLockFileEntry(lf, "skill-a", reg2)
	if found2 == nil {
		t.Fatal("Expected to find skill-a from reg2")
	}
	if found2.InstalledPath != "skill-a-owner-repo" {
		t.Errorf("Expected installedPath 'skill-a-owner-repo', got %q", found2.InstalledPath)
	}

	if storage.FindLockFileEntry(lf, "not-exist", reg1) != nil {
		t.Error("Expected nil for non-existent skill")
	}
}

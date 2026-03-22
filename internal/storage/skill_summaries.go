package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"agent-manager/internal/models"
)

// SummaryEntry represents a cached skill summary
type SummaryEntry struct {
	SkillName    string `json:"skill_name"`
	Registry     string `json:"registry"`
	RegistryType string `json:"registry_type"`
	Commit       string `json:"commit,omitempty"`
	Summary      string `json:"summary"`
}

// SkillSummariesCacheDir returns the directory for cached skill summaries
// On macOS: ~/Library/Caches/agent-manager/skill-summaries/
func SkillSummariesCacheDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, "Library", "Caches", "agent-manager", "skill-summaries")
}

// EnsureSkillSummariesCacheDir ensures the skill summaries cache directory exists
func EnsureSkillSummariesCacheDir() error {
	cacheDir := SkillSummariesCacheDir()
	return os.MkdirAll(cacheDir, 0755)
}

// getSummaryCacheKey generates a unique cache key for a skill
// Based on skill name, registry type, location, and commit (if available)
func getSummaryCacheKey(skill models.Skill, commit string) string {
	key := fmt.Sprintf("%s|%s|%s", skill.Name, skill.Registry.Type, skill.Registry.Location)
	if commit != "" {
		key = fmt.Sprintf("%s|%s", key, commit)
	}
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// getSummaryFilePath returns the path to the cache file for a skill summary
func getSummaryFilePath(skill models.Skill, commit string) string {
	cacheKey := getSummaryCacheKey(skill, commit)
	return filepath.Join(SkillSummariesCacheDir(), cacheKey+".json")
}

// GetCachedSummary retrieves a cached skill summary if it exists
func GetCachedSummary(skill models.Skill, commit string) (*SummaryEntry, error) {
	filePath := getSummaryFilePath(skill, commit)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var entry SummaryEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}

	return &entry, nil
}

// SaveSkillSummary saves a skill summary to the cache
func SaveSkillSummary(skill models.Skill, commit string, summary string) error {
	if err := EnsureSkillSummariesCacheDir(); err != nil {
		return err
	}

	entry := SummaryEntry{
		SkillName:    skill.Name,
		Registry:     skill.Registry.Location,
		RegistryType: string(skill.Registry.Type),
		Commit:       commit,
		Summary:      summary,
	}

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}

	filePath := getSummaryFilePath(skill, commit)
	return os.WriteFile(filePath, data, 0644)
}

// ClearSkillSummary removes a cached skill summary
func ClearSkillSummary(skill models.Skill, commit string) error {
	filePath := getSummaryFilePath(skill, commit)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// ClearAllSkillSummaries removes all cached skill summaries
func ClearAllSkillSummaries() error {
	cacheDir := SkillSummariesCacheDir()
	if err := os.RemoveAll(cacheDir); err != nil {
		return err
	}
	return EnsureSkillSummariesCacheDir()
}

// GetSkillCommit retrieves the commit hash for a skill from the lock file
func GetSkillCommit(lockFile *models.LockFile, skill models.Skill) string {
	for _, entry := range lockFile.Skills {
		if entry.Name == skill.Name &&
			entry.Registry.Type == skill.Registry.Type &&
			entry.Registry.Location == skill.Registry.Location {
			return entry.Commit
		}
	}
	return ""
}

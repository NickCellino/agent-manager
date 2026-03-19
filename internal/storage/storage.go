package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"agent-manager/internal/models"
)

// XDGDataDir returns the XDG data directory for agent-manager
func XDGDataDir() string {
	xdgDataHome := os.Getenv("XDG_DATA_HOME")
	if xdgDataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		xdgDataHome = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(xdgDataHome, "agent-manager")
}

// EnsureDataDir ensures the data directory exists
func EnsureDataDir() error {
	dataDir := XDGDataDir()
	return os.MkdirAll(dataDir, 0755)
}

// RegistryFilePath returns the path to the registry configuration file
func RegistryFilePath() string {
	return filepath.Join(XDGDataDir(), "registries.json")
}

// GitHubRegistriesDir returns the directory where GitHub registries are cloned
func GitHubRegistriesDir() string {
	return filepath.Join(XDGDataDir(), "github-registries")
}

// LoadRegistries loads the registry configuration from disk
func LoadRegistries() (*models.RegistryStore, error) {
	filePath := RegistryFilePath()

	// If file doesn't exist, return empty store
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return &models.RegistryStore{
			Registries: []models.Registry{},
		}, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read registries file: %w", err)
	}

	var store models.RegistryStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("failed to parse registries file: %w", err)
	}

	return &store, nil
}

// SaveRegistries saves the registry configuration to disk
func SaveRegistries(store *models.RegistryStore) error {
	if err := EnsureDataDir(); err != nil {
		return err
	}

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal registries: %w", err)
	}

	filePath := RegistryFilePath()
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write registries file: %w", err)
	}

	return nil
}

// AddRegistry adds a new registry to the store
func AddRegistry(store *models.RegistryStore, registry models.Registry) error {
	// Check for duplicates
	for _, r := range store.Registries {
		if r.Type == registry.Type && r.Location == registry.Location {
			return fmt.Errorf("registry with location '%s' already exists", registry.Location)
		}
	}

	store.Registries = append(store.Registries, registry)
	return SaveRegistries(store)
}

// RemoveRegistry removes a registry from the store
func RemoveRegistry(store *models.RegistryStore, registryType models.RegistryType, location string) error {
	for i, r := range store.Registries {
		if r.Type == registryType && r.Location == location {
			store.Registries = append(store.Registries[:i], store.Registries[i+1:]...)
			return SaveRegistries(store)
		}
	}
	return fmt.Errorf("registry '%s' (%s) not found", location, registryType)
}

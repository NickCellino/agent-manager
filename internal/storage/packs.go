package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"agent-manager/internal/models"
)

// PacksFilePath returns the path to the packs configuration file
func PacksFilePath() string {
	return filepath.Join(XDGDataDir(), "packs.json")
}

// LoadPacks loads the pack configuration from disk
func LoadPacks() (*models.PackStore, error) {
	filePath := PacksFilePath()

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return &models.PackStore{
			Packs: []models.Pack{},
		}, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read packs file: %w", err)
	}

	var store models.PackStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("failed to parse packs file: %w", err)
	}

	return &store, nil
}

// SavePacks saves the pack configuration to disk
func SavePacks(store *models.PackStore) error {
	if err := EnsureDataDir(); err != nil {
		return err
	}

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal packs: %w", err)
	}

	filePath := PacksFilePath()
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write packs file: %w", err)
	}

	return nil
}

// AddPack adds a new pack to the store
func AddPack(store *models.PackStore, pack models.Pack) error {
	for _, p := range store.Packs {
		if p.Name == pack.Name {
			return fmt.Errorf("pack %q already exists", pack.Name)
		}
	}
	store.Packs = append(store.Packs, pack)
	return SavePacks(store)
}

// RemovePack removes a pack from the store by name
func RemovePack(store *models.PackStore, name string) error {
	for i, p := range store.Packs {
		if p.Name == name {
			store.Packs = append(store.Packs[:i], store.Packs[i+1:]...)
			return SavePacks(store)
		}
	}
	return fmt.Errorf("pack %q not found", name)
}

// UpdatePack updates an existing pack in the store
func UpdatePack(store *models.PackStore, pack models.Pack) error {
	for i, p := range store.Packs {
		if p.Name == pack.Name {
			store.Packs[i] = pack
			return SavePacks(store)
		}
	}
	return fmt.Errorf("pack %q not found", pack.Name)
}

// FindPack looks up a pack by name; returns nil if not found
func FindPack(store *models.PackStore, name string) *models.Pack {
	for i := range store.Packs {
		if store.Packs[i].Name == name {
			return &store.Packs[i]
		}
	}
	return nil
}

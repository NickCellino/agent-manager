package tui

import (
	"testing"

	"agent-manager/internal/models"
)

func TestAgentsApplyFilterPreservesRegistryForDuplicateNames(t *testing.T) {
	model := &AgentsModel{
		allAgents: []models.Agent{
			{
				Name: "code-explainer",
				Registry: models.Registry{
					Type:     models.RegistryTypeGitHub,
					Location: "owner/first-registry",
				},
			},
			{
				Name: "code-explainer",
				Registry: models.Registry{
					Type:     models.RegistryTypeLocal,
					Location: "/tmp/second-registry",
				},
			},
		},
		filter: "code-explainer",
	}

	model.applyFilter()

	if len(model.filteredAgents) != 2 {
		t.Fatalf("expected 2 filtered agents, got %d", len(model.filteredAgents))
	}

	locations := map[string]bool{}
	for _, agent := range model.filteredAgents {
		locations[agent.Registry.Location] = true
	}

	if !locations["owner/first-registry"] {
		t.Fatalf("expected filtered agents to include first registry, got %#v", locations)
	}

	if !locations["/tmp/second-registry"] {
		t.Fatalf("expected filtered agents to include second registry, got %#v", locations)
	}
}

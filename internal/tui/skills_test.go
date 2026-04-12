package tui

import (
	"testing"

	"agent-manager/internal/models"
)

func TestSkillsApplyFilterPreservesRegistryForDuplicateNames(t *testing.T) {
	model := &SkillsModel{
		allSkills: []models.Skill{
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

	if len(model.filteredSkills) != 2 {
		t.Fatalf("expected 2 filtered skills, got %d", len(model.filteredSkills))
	}

	locations := map[string]bool{}
	for _, skill := range model.filteredSkills {
		locations[skill.Registry.Location] = true
	}

	if !locations["owner/first-registry"] {
		t.Fatalf("expected filtered skills to include first registry, got %#v", locations)
	}

	if !locations["/tmp/second-registry"] {
		t.Fatalf("expected filtered skills to include second registry, got %#v", locations)
	}
}

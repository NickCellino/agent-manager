package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

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

func TestSkillsVisibleRangeTracksTopMiddleAndEnd(t *testing.T) {
	model := testSkillsModel(10, 4)

	start, end := model.visibleSkillRange()
	if start != 0 || end != 4 {
		t.Fatalf("expected top range 0:4, got %d:%d", start, end)
	}

	model.cursor = 5
	model.syncSkillViewport()
	start, end = model.visibleSkillRange()
	if start != 2 || end != 6 {
		t.Fatalf("expected middle range 2:6, got %d:%d", start, end)
	}

	model.cursor = 9
	model.syncSkillViewport()
	start, end = model.visibleSkillRange()
	if start != 6 || end != 10 {
		t.Fatalf("expected end range 6:10, got %d:%d", start, end)
	}
}

func TestSkillsNavigationKeepsCursorVisible(t *testing.T) {
	model := testSkillsModel(8, 3)

	for range 5 {
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
		model = updated.(*SkillsModel)
	}

	if model.cursor != 5 {
		t.Fatalf("expected cursor at 5, got %d", model.cursor)
	}

	if model.listViewport != 3 {
		t.Fatalf("expected viewport to scroll to 3, got %d", model.listViewport)
	}

	view := model.viewSelect()
	if strings.Contains(view, "skill-2") {
		t.Fatalf("expected off-screen skill-2 to be hidden, view was:\n%s", view)
	}
	if !strings.Contains(view, "> [ ] skill-5") {
		t.Fatalf("expected selected skill-5 to remain visible, view was:\n%s", view)
	}

	for range 4 {
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyUp})
		model = updated.(*SkillsModel)
	}

	if model.cursor != 1 {
		t.Fatalf("expected cursor at 1 after moving back up, got %d", model.cursor)
	}

	if model.listViewport != 1 {
		t.Fatalf("expected viewport to scroll back to 1, got %d", model.listViewport)
	}
}

func TestSkillsResizeRecomputesViewport(t *testing.T) {
	model := testSkillsModel(10, 5)
	model.cursor = 9
	model.syncSkillViewport()

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 8})
	model = updated.(*SkillsModel)

	if model.visibleSkillRows() != 2 {
		t.Fatalf("expected 2 visible rows after resize, got %d", model.visibleSkillRows())
	}

	if model.listViewport != 8 {
		t.Fatalf("expected viewport to clamp to 8 after resize, got %d", model.listViewport)
	}

	start, end := model.visibleSkillRange()
	if start != 8 || end != 10 {
		t.Fatalf("expected resized range 8:10, got %d:%d", start, end)
	}
	if model.cursor != 9 {
		t.Fatalf("expected cursor to remain on last skill, got %d", model.cursor)
	}
}

func TestSkillsVisibleRangeHandlesShortLists(t *testing.T) {
	model := testSkillsModel(2, 6)

	start, end := model.visibleSkillRange()
	if start != 0 || end != 2 {
		t.Fatalf("expected short list range 0:2, got %d:%d", start, end)
	}

	view := model.viewSelect()
	if !strings.Contains(view, "skill-0") || !strings.Contains(view, "skill-1") {
		t.Fatalf("expected both short-list skills to render, view was:\n%s", view)
	}
}

func TestSkillsPageStrideNavigationClampsAtBounds(t *testing.T) {
	model := testSkillsModel(10, 4)

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	model = updated.(*SkillsModel)

	if model.cursor != 4 {
		t.Fatalf("expected lowercase l to move cursor to 4, got %d", model.cursor)
	}

	if model.listViewport != 1 {
		t.Fatalf("expected viewport to follow paged cursor to 1, got %d", model.listViewport)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	model = updated.(*SkillsModel)

	if model.cursor != 8 {
		t.Fatalf("expected uppercase L to move cursor to 8, got %d", model.cursor)
	}

	if model.listViewport != 5 {
		t.Fatalf("expected viewport to follow paged cursor to 5, got %d", model.listViewport)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	model = updated.(*SkillsModel)

	if model.cursor != 9 {
		t.Fatalf("expected forward paging to clamp at last item, got %d", model.cursor)
	}

	if model.listViewport != 6 {
		t.Fatalf("expected viewport to clamp at 6, got %d", model.listViewport)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'H'}})
	model = updated.(*SkillsModel)

	if model.cursor != 5 {
		t.Fatalf("expected uppercase H to move cursor back to 5, got %d", model.cursor)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	model = updated.(*SkillsModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	model = updated.(*SkillsModel)

	if model.cursor != 0 {
		t.Fatalf("expected backward paging to clamp at first item, got %d", model.cursor)
	}

	if model.listViewport != 0 {
		t.Fatalf("expected viewport to clamp back to 0, got %d", model.listViewport)
	}
}

func TestSkillsFilterModeKeepsHLAsTextInput(t *testing.T) {
	model := testSkillsModel(6, 3)
	model.inputMode = "filter"
	model.textInput.Focus()

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	model = updated.(*SkillsModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	model = updated.(*SkillsModel)

	if model.textInput.Value() != "hl" {
		t.Fatalf("expected filter input to capture hl, got %q", model.textInput.Value())
	}

	if model.cursor != 0 {
		t.Fatalf("expected filter typing not to page cursor, got %d", model.cursor)
	}

	if model.listViewport != 0 {
		t.Fatalf("expected filter typing not to change viewport, got %d", model.listViewport)
	}
}

func testSkillsModel(skillCount, visibleRows int) *SkillsModel {
	allSkills := make([]models.Skill, 0, skillCount)
	for i := range skillCount {
		allSkills = append(allSkills, models.Skill{
			Name: fmt.Sprintf("skill-%d", i),
			Registry: models.Registry{
				Type:     models.RegistryTypeLocal,
				Location: fmt.Sprintf("/tmp/registry-%d", i),
			},
		})
	}

	ti := textinput.New()
	ti.Placeholder = "type to filter..."
	ti.CharLimit = 156
	ti.Prompt = ""

	model := &SkillsModel{
		allSkills:      allSkills,
		filteredSkills: allSkills,
		selectedSkills: map[string]bool{},
		textInput:      ti,
		height:         visibleRows + 6,
		inputMode:      "navigate",
		mode:           "select",
	}
	model.syncSkillViewport()

	return model
}

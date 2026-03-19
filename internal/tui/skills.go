package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"

	"agent-manager/internal/models"
	"agent-manager/internal/skills"
)

// Shared styles (also defined in registry.go)
var (
	skillsTitleStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FAFAFA")).Background(lipgloss.Color("#7D56F4")).Padding(0, 1)
	skillsItemStyle         = lipgloss.NewStyle()
	skillsSelectedItemStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("170"))
	skillsHelpStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

// SkillsModel manages the skills selection TUI
type SkillsModel struct {
	allSkills       []models.Skill
	filteredSkills  []models.Skill
	selectedSkills  map[string]bool
	installedSkills map[string]bool // Skills already in .opencode/skills
	textInput       textinput.Model
	cursor          int
	filter          string
	mode            string // "select", "confirm-delete"
	skillsToDelete  []string
	err             error
	width           int
	height          int
	done            bool
	saved           bool
}

// NewSkillsModel creates a new skills selection model
func NewSkillsModel() (*SkillsModel, error) {
	// Discover all skills from registries
	allSkills, err := skills.DiscoverSkills()
	if err != nil {
		return nil, err
	}

	// Get currently installed skills
	installed, err := skills.ListInstalledSkills()
	if err != nil {
		return nil, err
	}

	installedSkills := make(map[string]bool)
	selectedSkills := make(map[string]bool)

	for _, skillName := range installed {
		installedSkills[skillName] = true
		// Pre-select installed skills
		selectedSkills[skillName] = true
	}

	ti := textinput.New()
	ti.Placeholder = "type to filter..."
	ti.Focus()
	ti.CharLimit = 156
	ti.Cursor.Style = lipgloss.NewStyle()
	ti.Prompt = ""

	return &SkillsModel{
		allSkills:       allSkills,
		filteredSkills:  allSkills,
		selectedSkills:  selectedSkills,
		installedSkills: installedSkills,
		textInput:       ti,
		mode:            "select",
	}, nil
}

func (m *SkillsModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *SkillsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.textInput.Width = msg.Width - 20

	case tea.KeyMsg:
		switch m.mode {
		case "select":
			return m.updateSelectMode(msg)
		case "confirm-delete":
			return m.updateConfirmDeleteMode(msg)
		}
	}

	// Update text input for filtering
	if m.mode == "select" {
		var cmd tea.Cmd
		oldValue := m.textInput.Value()
		m.textInput, cmd = m.textInput.Update(msg)

		// If filter changed, update filtered skills
		if m.textInput.Value() != oldValue {
			m.filter = m.textInput.Value()
			m.applyFilter()
		}

		return m, cmd
	}

	return m, nil
}

func (m *SkillsModel) updateSelectMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		m.done = true
		return m, tea.Quit

	case tea.KeyEnter:
		// Save selections
		if err := m.saveSelections(); err != nil {
			m.err = err
			return m, tea.Quit
		}

		// Check if we need to confirm deletions
		if len(m.skillsToDelete) > 0 {
			m.mode = "confirm-delete"
			return m, nil
		}

		m.saved = true
		m.done = true
		return m, tea.Quit

	case tea.KeySpace:
		// Toggle selection
		if len(m.filteredSkills) > 0 && m.cursor < len(m.filteredSkills) {
			skill := m.filteredSkills[m.cursor]
			if m.selectedSkills[skill.Name] {
				delete(m.selectedSkills, skill.Name)
			} else {
				m.selectedSkills[skill.Name] = true
			}
		}
		return m, nil

	case tea.KeyUp, tea.KeyDown:
		// Navigate with arrow keys
		if msg.Type == tea.KeyUp && m.cursor > 0 {
			m.cursor--
		} else if msg.Type == tea.KeyDown && m.cursor < len(m.filteredSkills)-1 {
			m.cursor++
		}
		return m, nil

	case tea.KeyRunes:
		// Vim-style navigation (j/k)
		switch string(msg.Runes) {
		case "k":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "j":
			if m.cursor < len(m.filteredSkills)-1 {
				m.cursor++
			}
			return m, nil
		}

	case tea.KeyEsc:
		m.done = true
		return m, tea.Quit
	}

	return m, nil
}

func (m *SkillsModel) updateConfirmDeleteMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		// Delete the skills
		skillsDir, err := skills.EnsureProjectSkillsDir()
		if err != nil {
			m.err = err
			return m, tea.Quit
		}

		for _, skillName := range m.skillsToDelete {
			if err := skills.RemoveSkill(skillName, skillsDir); err != nil {
				m.err = err
				return m, tea.Quit
			}
		}

		m.saved = true
		m.done = true
		return m, tea.Quit

	case "n", "N", "esc":
		// Cancel deletion
		m.done = true
		return m, tea.Quit
	}

	return m, nil
}

func (m *SkillsModel) applyFilter() {
	if m.filter == "" {
		m.filteredSkills = m.allSkills
		m.cursor = 0
		return
	}

	// Use fuzzy matching
	var names []string
	for _, skill := range m.allSkills {
		names = append(names, skill.Name)
	}

	matches := fuzzy.Find(m.filter, names)
	m.filteredSkills = nil
	for _, match := range matches {
		for _, skill := range m.allSkills {
			if skill.Name == match.Str {
				m.filteredSkills = append(m.filteredSkills, skill)
				break
			}
		}
	}
	m.cursor = 0
}

func (m *SkillsModel) saveSelections() error {
	skillsDir, err := skills.EnsureProjectSkillsDir()
	if err != nil {
		return err
	}

	// Install selected skills
	for _, skill := range m.allSkills {
		if m.selectedSkills[skill.Name] {
			if err := skills.InstallSkill(skill, skillsDir); err != nil {
				return err
			}
		}
	}

	// Check which installed skills are no longer selected
	for skillName := range m.installedSkills {
		if !m.selectedSkills[skillName] {
			// Check if this skill is from a registry (has a matching skill in allSkills)
			isFromRegistry := false
			for _, skill := range m.allSkills {
				if skill.Name == skillName {
					isFromRegistry = true
					break
				}
			}
			if isFromRegistry {
				m.skillsToDelete = append(m.skillsToDelete, skillName)
			}
		}
	}

	return nil
}

func (m *SkillsModel) View() string {
	switch m.mode {
	case "select":
		return m.viewSelect()
	case "confirm-delete":
		return m.viewConfirmDelete()
	default:
		return ""
	}
}

func (m *SkillsModel) viewSelect() string {
	var b strings.Builder

	// Title
	b.WriteString(skillsTitleStyle.Render("Select Skills") + "\n")

	// Filter input
	b.WriteString("Filter: " + m.textInput.View() + "\n")

	// Skill list
	if len(m.filteredSkills) == 0 {
		b.WriteString("  No skills found\n")
	} else {
		for i, skill := range m.filteredSkills {
			cursor := "  "
			if i == m.cursor {
				cursor = "> "
			}

			checked := "[ ]"
			if m.selectedSkills[skill.Name] {
				checked = "[x]"
			}

			line := fmt.Sprintf("%s%s %s (%s)", cursor, checked, skill.Name, skill.Registry)

			if i == m.cursor {
				b.WriteString(skillsSelectedItemStyle.Render(line) + "\n")
			} else {
				b.WriteString(skillsItemStyle.Render(line) + "\n")
			}
		}
	}

	// Stats
	b.WriteString(fmt.Sprintf("\n%d/%d skills selected\n", len(m.selectedSkills), len(m.allSkills)))

	// Help
	b.WriteString("\n" + skillsHelpStyle.Render("↑/↓/j/k: navigate  space: toggle  enter: save  esc: cancel"))

	return b.String()
}

func (m *SkillsModel) viewConfirmDelete() string {
	var b strings.Builder

	b.WriteString(skillsTitleStyle.Render("Confirm Deletion") + "\n\n")
	b.WriteString("The following skills will be removed:\n\n")

	for _, skillName := range m.skillsToDelete {
		b.WriteString(fmt.Sprintf("  - %s\n", skillName))
	}

	b.WriteString("\n" + skillsHelpStyle.Render("y: confirm  n: cancel"))

	return b.String()
}

// RunSkillsTUI runs the skills selection TUI
func RunSkillsTUI() (bool, error) {
	model, err := NewSkillsModel()
	if err != nil {
		return false, err
	}

	p := tea.NewProgram(model)
	m, err := p.Run()
	if err != nil {
		return false, err
	}

	result := m.(*SkillsModel)

	if result.err != nil {
		return false, result.err
	}

	return result.saved, nil
}

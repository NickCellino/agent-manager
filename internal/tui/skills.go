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
	"agent-manager/internal/storage"
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
	installedSkills map[string]bool  // Skills already in .opencode/skills (by installedPath)
	lockFile        *models.LockFile // Lock file for managed skills
	textInput       textinput.Model
	cursor          int
	filter          string
	mode            string // "select", "confirm-delete"
	inputMode       string // "navigate", "filter"
	skillsToDelete  []string
	err             error
	width           int
	height          int
	done            bool
	saved           bool
}

// formatRegistryDisplay formats a registry for display
// Shows last 2 components of location, truncated to 40 chars, colored
func formatRegistryDisplay(registry models.Registry) string {
	location := registry.Location

	// Get last 2 components
	parts := strings.Split(location, "/")
	if len(parts) >= 2 {
		location = parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}

	// Truncate to 40 chars
	if len(location) > 40 {
		location = location[:37] + "..."
	}

	// Color based on registry type
	color := "39" // default blue
	if registry.Type == models.RegistryTypeGitHub {
		color = "208" // orange for GitHub
	} else {
		color = "82" // green for local
	}

	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(fmt.Sprintf("(%s)", location))
}

// NewSkillsModel creates a new skills selection model
func NewSkillsModel() (*SkillsModel, error) {
	// Discover all skills from registries
	allSkills, err := skills.DiscoverSkills()
	if err != nil {
		return nil, err
	}

	// Load lock file - this is the source of truth for what skills are selected
	lockFile, err := storage.LoadLockFile()
	if err != nil {
		return nil, err
	}

	// Get currently installed skills from filesystem
	// We only use this to detect unmanaged skills that exist but aren't tracked
	installed, err := skills.ListInstalledSkills()
	if err != nil {
		return nil, err
	}

	// Track installed skills by their path name
	installedSkills := make(map[string]bool)
	for _, skillPath := range installed {
		installedSkills[skillPath] = true
	}

	// Track selected skills - keyed by "skillName|registryType|registryLocation"
	// The lock file is the source of truth - only select skills that are in the lock file
	selectedSkills := make(map[string]bool)

	// Mark skills from lock file as selected
	for _, entry := range lockFile.Skills {
		key := fmt.Sprintf("%s|%s|%s", entry.Name, entry.Registry.Type, entry.Registry.Location)
		selectedSkills[key] = true
	}

	ti := textinput.New()
	ti.Placeholder = "type to filter..."
	// Don't focus by default - user must press '/' to activate filter
	ti.CharLimit = 156
	ti.Cursor.Style = lipgloss.NewStyle()
	ti.Prompt = ""

	return &SkillsModel{
		allSkills:       allSkills,
		filteredSkills:  allSkills,
		selectedSkills:  selectedSkills,
		installedSkills: installedSkills,
		lockFile:        lockFile,
		textInput:       ti,
		mode:            "select",
		inputMode:       "navigate",
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
			// Handle keys based on input mode
			if m.inputMode == "filter" {
				// In filter mode: ESC exits filter, everything else goes to textinput
				switch msg.Type {
				case tea.KeyEsc:
					m.inputMode = "navigate"
					m.textInput.Blur()
					return m, nil
				default:
					// Pass to text input
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
			} else {
				// In navigate mode: handle navigation keys
				return m.updateNavigateMode(msg)
			}
		case "confirm-delete":
			return m.updateConfirmDeleteMode(msg)
		}
	}

	return m, nil
}

func (m *SkillsModel) updateNavigateMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
			skillKey := fmt.Sprintf("%s|%s|%s", skill.Name, skill.Registry.Type, skill.Registry.Location)
			if m.selectedSkills[skillKey] {
				delete(m.selectedSkills, skillKey)
			} else {
				m.selectedSkills[skillKey] = true
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

	case tea.KeyEsc:
		// ESC in navigate mode quits
		m.done = true
		return m, tea.Quit

	case tea.KeyRunes:
		// Check for special keys first
		if len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case '/':
				// Activate filter mode
				m.inputMode = "filter"
				m.textInput.Focus()
				return m, textinput.Blink
			case 'k':
				if m.cursor > 0 {
					m.cursor--
				}
				return m, nil
			case 'j':
				if m.cursor < len(m.filteredSkills)-1 {
					m.cursor++
				}
				return m, nil
			}
		}
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

		for _, skillPath := range m.skillsToDelete {
			// Remove from filesystem
			if err := skills.RemoveSkill(skillPath, skillsDir); err != nil {
				m.err = err
				return m, tea.Quit
			}

			// Remove from lock file
			entry := storage.GetManagedSkillEntry(m.lockFile, skillPath)
			if entry != nil {
				if err := storage.RemoveSkillFromLockFile(m.lockFile, entry.Name, entry.Registry); err != nil {
					// Log but continue
					fmt.Printf("Warning: failed to remove %s from lock file: %v\n", entry.Name, err)
				}
			}
		}

		m.saved = true
		m.done = true
		return m, tea.Quit

	case "n", "N", "esc":
		// Cancel deletion - don't remove from lock file
		m.saved = true
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

	// Build a list of names indexed the same way as m.allSkills for fuzzy matching.
	// Using match.Index lets us correctly retrieve duplicate-named skills from
	// different registries without the first-match-wins break.
	var names []string
	for _, skill := range m.allSkills {
		names = append(names, skill.Name)
	}

	matches := fuzzy.Find(m.filter, names)
	m.filteredSkills = nil
	for _, match := range matches {
		m.filteredSkills = append(m.filteredSkills, m.allSkills[match.Index])
	}
	m.cursor = 0
}

func (m *SkillsModel) saveSelections() error {
	skillsDir, err := skills.EnsureProjectSkillsDir()
	if err != nil {
		return err
	}

	// Install newly selected skills (not already in lock file)
	for _, skill := range m.allSkills {
		skillKey := fmt.Sprintf("%s|%s|%s", skill.Name, skill.Registry.Type, skill.Registry.Location)
		if m.selectedSkills[skillKey] {
			// Check if already installed (managed)
			existingEntry := storage.FindLockFileEntry(m.lockFile, skill.Name, skill.Registry)

			// Skip if already managed - no need to reinstall
			if existingEntry != nil {
				continue
			}

			// New skill - generate unique path (checking both lock file and filesystem)
			installedPath := storage.GenerateInstalledPath(skill.Name, skill.Registry, m.lockFile, skillsDir)

			// Install the skill with the target path
			if err := skills.InstallSkill(skill, skillsDir, installedPath); err != nil {
				return fmt.Errorf("failed to install skill %s: %w", skill.Name, err)
			}

			// Get commit hash for GitHub registries
			var commit string
			if skill.Registry.Type == models.RegistryTypeGitHub {
				registryPath := skills.GetGitHubRegistryPath(skill.Registry.Location)
				commit, _ = storage.GetGitCommit(registryPath)
			}

			// Add to lock file
			entry := models.LockFileEntry{
				Name:          skill.Name,
				InstalledPath: installedPath,
				Registry:      skill.Registry,
				Commit:        commit,
			}
			if err := storage.AddSkillToLockFile(m.lockFile, entry); err != nil {
				return fmt.Errorf("failed to update lock file for skill %s: %w", skill.Name, err)
			}
		}
	}

	// Check which installed skills are no longer selected
	// Only consider managed skills (those in lock file)
	for skillPath := range m.installedSkills {
		// Check if this is a managed skill
		entry := storage.GetManagedSkillEntry(m.lockFile, skillPath)
		if entry == nil {
			// Not managed - skip
			continue
		}

		// Check if still selected
		skillKey := fmt.Sprintf("%s|%s|%s", entry.Name, entry.Registry.Type, entry.Registry.Location)
		if !m.selectedSkills[skillKey] {
			m.skillsToDelete = append(m.skillsToDelete, skillPath)
			// Don't remove from lock file yet - wait for confirmation
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

	// Filter input - show focused state
	if m.inputMode == "filter" {
		b.WriteString("Filter: " + m.textInput.View() + "\n")
	} else {
		// Show placeholder when not in filter mode
		if m.textInput.Value() == "" {
			b.WriteString("Filter: " + skillsHelpStyle.Render("press / to filter") + "\n")
		} else {
			b.WriteString("Filter: " + m.textInput.View() + "\n")
		}
	}

	// Skill list
	if len(m.filteredSkills) == 0 {
		b.WriteString("  No skills found\n")
	} else {
		for i, skill := range m.filteredSkills {
			cursor := "  "
			if i == m.cursor {
				cursor = "> "
			}

			skillKey := fmt.Sprintf("%s|%s|%s", skill.Name, skill.Registry.Type, skill.Registry.Location)
			checked := "[ ]"
			if m.selectedSkills[skillKey] {
				checked = "[x]"
			}

			line := fmt.Sprintf("%s%s %s %s", cursor, checked, skill.Name, formatRegistryDisplay(skill.Registry))

			if i == m.cursor {
				b.WriteString(skillsSelectedItemStyle.Render(line) + "\n")
			} else {
				b.WriteString(skillsItemStyle.Render(line) + "\n")
			}
		}
	}

	// Stats
	b.WriteString(fmt.Sprintf("\n%d/%d skills selected\n", len(m.selectedSkills), len(m.allSkills)))

	// Help - different based on input mode
	if m.inputMode == "filter" {
		b.WriteString("\n" + skillsHelpStyle.Render("esc: exit filter  type to search"))
	} else {
		b.WriteString("\n" + skillsHelpStyle.Render("/: filter  ↑/↓/j/k: navigate  space: toggle  enter: save  esc: cancel"))
	}

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

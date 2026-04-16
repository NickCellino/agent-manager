package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"

	"agent-manager/internal/models"
	"agent-manager/internal/skills"
	"agent-manager/internal/storage"
)

// Shared list styles used by both the skills and agents TUIs
var (
	listTitleStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FAFAFA")).Background(lipgloss.Color("#7D56F4")).Padding(0, 1)
	listItemStyle         = lipgloss.NewStyle()
	listSelectedItemStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("170"))
	listHelpStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
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
	listViewport    int
	filter          string
	mode            string // "select", "confirm-delete", "skill-summary", "skill-summary-loading"
	inputMode       string // "navigate", "filter"
	skillsToDelete  []string
	err             error
	width           int
	height          int
	done            bool
	saved           bool
	// Skill summary view fields
	summarySpinner  spinner.Model
	summaryContent  string
	summaryViewport int // Scroll position for summary
	summarySkill    models.Skill
	summaryGlamour  *glamour.TermRenderer
}

// formatRegistryDisplay formats a registry for display.
// Shows registry type plus the last 2 path components, truncated to 40 chars.
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

	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(fmt.Sprintf("(%s: %s)", registry.Type, location))
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

	// Initialize spinner for loading states
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return &SkillsModel{
		allSkills:       allSkills,
		filteredSkills:  allSkills,
		selectedSkills:  selectedSkills,
		installedSkills: installedSkills,
		lockFile:        lockFile,
		textInput:       ti,
		mode:            "select",
		inputMode:       "navigate",
		summarySpinner:  s,
	}, nil
}

func (m *SkillsModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.summarySpinner.Tick)
}

func (m *SkillsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.textInput.Width = msg.Width - 20
		m.syncSkillViewport()

		// Re-initialize glamour renderer if in summary mode
		if m.mode == "skill-summary" && m.summaryGlamour != nil {
			m.initGlamourRenderer()
			m.renderSummary()
		}

	case summaryLoadedMsg:
		// Handle loaded summary
		if msg.err != nil {
			m.summaryContent = fmt.Sprintf("## %s\n\n**Error loading summary:** %v\n\n*Press 'q' to go back.*", m.summarySkill.Name, msg.err)
		} else {
			m.summaryContent = msg.content
		}
		m.mode = "skill-summary"
		m.initGlamourRenderer() // Initialize glamour before rendering
		return m, nil

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
		case "skill-summary-loading", "skill-summary":
			return m.updateSummaryMode(msg)
		}

	case spinner.TickMsg:
		// Update spinner for loading states
		if m.mode == "skill-summary-loading" {
			var cmd tea.Cmd
			m.summarySpinner, cmd = m.summarySpinner.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

// summaryLoadedMsg is sent when a skill summary has been loaded/generated
type summaryLoadedMsg struct {
	content string
	err     error
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
		m.syncSkillViewport()
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
			case 'h', 'H':
				m.pageSkills(-m.visibleSkillRows())
				return m, nil
			case 'l', 'L':
				m.pageSkills(m.visibleSkillRows())
				return m, nil
			case 'k':
				if m.cursor > 0 {
					m.cursor--
				}
				m.syncSkillViewport()
				return m, nil
			case 'j':
				if m.cursor < len(m.filteredSkills)-1 {
					m.cursor++
				}
				m.syncSkillViewport()
				return m, nil
			case 'i':
				// Show skill info/summary
				if len(m.filteredSkills) > 0 && m.cursor < len(m.filteredSkills) {
					skill := m.filteredSkills[m.cursor]
					return m.loadSkillSummary(skill)
				}
				return m, nil
			}
		}
	}

	return m, nil
}

func (m *SkillsModel) pageSkills(delta int) {
	if len(m.filteredSkills) == 0 || delta == 0 {
		return
	}

	m.cursor += delta
	m.syncSkillViewport()
}

// loadSkillSummary loads or generates a summary for the given skill
func (m *SkillsModel) loadSkillSummary(skill models.Skill) (tea.Model, tea.Cmd) {
	m.summarySkill = skill
	m.summaryContent = ""
	m.summaryViewport = 0

	// Check cache first
	commit := storage.GetSkillCommit(m.lockFile, skill)
	if cached, err := storage.GetCachedSummary(skill, commit); err == nil && cached != nil {
		// Use cached summary
		m.summaryContent = cached.Summary
		m.mode = "skill-summary"
		m.initGlamourRenderer()
		return m, nil
	}

	// Need to generate summary - show loading state
	m.mode = "skill-summary-loading"

	// Load/generate summary asynchronously, and start spinner ticking
	return m, tea.Batch(
		m.summarySpinner.Tick,
		func() tea.Msg {
			// Try to generate summary
			summary, err := skills.GenerateSkillSummary(skill)
			if err != nil {
				return summaryLoadedMsg{err: err}
			}

			// Cache the summary
			if err := storage.SaveSkillSummary(skill, commit, summary); err != nil {
				// Non-fatal: just log the error
				fmt.Fprintf(os.Stderr, "Warning: failed to cache skill summary: %v\n", err)
			}

			return summaryLoadedMsg{content: summary}
		},
	)
}

// initGlamourRenderer initializes the glamour markdown renderer
func (m *SkillsModel) initGlamourRenderer() {
	if m.summaryGlamour != nil {
		m.summaryGlamour.Close()
	}

	width := m.width - 4
	if width < 20 {
		width = 80
	}

	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		// If glamour fails, we'll fall back to plain text
		m.summaryGlamour = nil
		return
	}
	m.summaryGlamour = renderer
}

// renderSummary renders the markdown summary using glamour
func (m *SkillsModel) renderSummary() string {
	if m.summaryGlamour == nil || m.summaryContent == "" {
		return m.summaryContent
	}

	rendered, err := m.summaryGlamour.Render(m.summaryContent)
	if err != nil {
		// Fall back to plain text
		return m.summaryContent
	}
	return rendered
}

func (m *SkillsModel) updateSummaryMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		// Return to skills list
		m.mode = "select"
		m.summaryContent = ""
		m.summaryViewport = 0
		if m.summaryGlamour != nil {
			m.summaryGlamour.Close()
			m.summaryGlamour = nil
		}
		return m, nil
	case "up", "k":
		// Scroll up
		if m.summaryViewport > 0 {
			m.summaryViewport--
		}
		return m, nil
	case "down", "j":
		// Scroll down (simple implementation - could be improved)
		m.summaryViewport++
		return m, nil
	}

	return m, nil
}

func (m *SkillsModel) updateConfirmDeleteMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		for _, skillPath := range m.skillsToDelete {
			entry := storage.GetManagedSkillEntry(m.lockFile, skillPath)
			if entry == nil {
				continue
			}
			if err := skills.RemoveSkillFromProject(entry, m.lockFile); err != nil {
				// Log but continue — don't abort if one removal fails
				fmt.Printf("Warning: failed to remove %s: %v\n", entry.Name, err)
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
		m.listViewport = 0
		m.syncSkillViewport()
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
		if match.Index >= 0 && match.Index < len(m.allSkills) {
			m.filteredSkills = append(m.filteredSkills, m.allSkills[match.Index])
		}
	}
	m.cursor = 0
	m.listViewport = 0
	m.syncSkillViewport()
}

func (m *SkillsModel) visibleSkillRows() int {
	if m.height <= 0 {
		if len(m.filteredSkills) == 0 {
			return 1
		}
		return len(m.filteredSkills)
	}

	rows := m.height - 6
	if rows < 1 {
		return 1
	}

	return rows
}

func (m *SkillsModel) visibleSkillRange() (int, int) {
	if len(m.filteredSkills) == 0 {
		return 0, 0
	}

	m.syncSkillViewport()

	start := m.listViewport
	end := start + m.visibleSkillRows()
	if end > len(m.filteredSkills) {
		end = len(m.filteredSkills)
	}

	return start, end
}

func (m *SkillsModel) visibleSkillRangeSummary() string {
	if len(m.filteredSkills) == 0 {
		return "0 of 0 shown"
	}

	start, end := m.visibleSkillRange()
	return fmt.Sprintf("%d-%d of %d shown", start+1, end, len(m.filteredSkills))
}

func (m *SkillsModel) syncSkillViewport() {
	if len(m.filteredSkills) == 0 {
		m.cursor = 0
		m.listViewport = 0
		return
	}

	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.filteredSkills) {
		m.cursor = len(m.filteredSkills) - 1
	}

	rows := m.visibleSkillRows()
	maxViewport := len(m.filteredSkills) - rows
	if maxViewport < 0 {
		maxViewport = 0
	}

	if m.listViewport < 0 {
		m.listViewport = 0
	}
	if m.listViewport > maxViewport {
		m.listViewport = maxViewport
	}

	if m.cursor < m.listViewport {
		m.listViewport = m.cursor
	}
	if m.cursor >= m.listViewport+rows {
		m.listViewport = m.cursor - rows + 1
	}

	if m.listViewport > maxViewport {
		m.listViewport = maxViewport
	}
}

func (m *SkillsModel) saveSelections() error {
	// Install newly selected skills (not already in lock file)
	for _, skill := range m.allSkills {
		skillKey := fmt.Sprintf("%s|%s|%s", skill.Name, skill.Registry.Type, skill.Registry.Location)
		if m.selectedSkills[skillKey] {
			// Skip if already managed - no need to reinstall
			if storage.FindLockFileEntry(m.lockFile, skill.Name, skill.Registry) != nil {
				continue
			}

			if _, err := skills.AddSkillToProject(skill, m.lockFile); err != nil {
				return err
			}
		}
	}

	// Check which installed skills are no longer selected
	// Only consider managed skills (those in lock file)
	for skillPath := range m.installedSkills {
		entry := storage.GetManagedSkillEntry(m.lockFile, skillPath)
		if entry == nil {
			// Not managed - skip
			continue
		}

		skillKey := fmt.Sprintf("%s|%s|%s", entry.Name, entry.Registry.Type, entry.Registry.Location)
		if !m.selectedSkills[skillKey] {
			m.skillsToDelete = append(m.skillsToDelete, skillPath)
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
	case "skill-summary-loading":
		return m.viewSummaryLoading()
	case "skill-summary":
		return m.viewSummary()
	default:
		return ""
	}
}

func (m *SkillsModel) viewSelect() string {
	var b strings.Builder

	// Title
	b.WriteString(listTitleStyle.Render("Select Skills") + "\n")

	// Filter input - show focused state
	if m.inputMode == "filter" {
		b.WriteString("Filter: " + m.textInput.View() + "\n")
	} else {
		// Show placeholder when not in filter mode
		if m.textInput.Value() == "" {
			b.WriteString("Filter: " + listHelpStyle.Render("press / to filter") + "\n")
		} else {
			b.WriteString("Filter: " + m.textInput.View() + "\n")
		}
	}

	// Skill list
	if len(m.filteredSkills) == 0 {
		b.WriteString("  No skills found\n")
	} else {
		start, end := m.visibleSkillRange()
		for i := start; i < end; i++ {
			skill := m.filteredSkills[i]
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
				b.WriteString(listSelectedItemStyle.Render(line) + "\n")
			} else {
				b.WriteString(listItemStyle.Render(line) + "\n")
			}
		}
	}

	// Stats
	b.WriteString("\n" + m.visibleSkillRangeSummary() + "\n")
	b.WriteString(fmt.Sprintf("%d/%d skills selected\n", len(m.selectedSkills), len(m.allSkills)))

	// Help - different based on input mode
	if m.inputMode == "filter" {
		b.WriteString("\n" + listHelpStyle.Render("esc: exit filter  type to search"))
	} else {
		b.WriteString("\n" + listHelpStyle.Render("/: filter  ↑/↓/j/k: navigate  h/l: page  space: toggle  i: info  enter: save  esc: cancel"))
	}

	return b.String()
}

func (m *SkillsModel) viewConfirmDelete() string {
	var b strings.Builder

	b.WriteString(listTitleStyle.Render("Confirm Deletion") + "\n\n")
	b.WriteString("The following skills will be removed:\n\n")

	for _, skillName := range m.skillsToDelete {
		b.WriteString(fmt.Sprintf("  - %s\n", skillName))
	}

	b.WriteString("\n" + listHelpStyle.Render("y: confirm  n: cancel"))

	return b.String()
}

func (m *SkillsModel) viewSummaryLoading() string {
	var b strings.Builder

	// Title bar
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	b.WriteString(titleStyle.Render(fmt.Sprintf(" Skill: %s ", m.summarySkill.Name)) + "\n\n")

	// Loading spinner
	b.WriteString(fmt.Sprintf("  %s Loading skill summary...\n", m.summarySpinner.View()))

	// Help
	b.WriteString("\n" + listHelpStyle.Render("esc: cancel"))

	return b.String()
}

func (m *SkillsModel) viewSummary() string {
	var b strings.Builder

	// Title bar
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	b.WriteString(titleStyle.Render(fmt.Sprintf(" Skill: %s ", m.summarySkill.Name)) + "\n")

	// Ensure glamour renderer is initialized
	if m.summaryGlamour == nil {
		m.initGlamourRenderer()
	}

	// Render the markdown content (or use plain text if glamour fails)
	contentToDisplay := m.renderSummary()

	// Content - split into lines and show viewport window
	lines := strings.Split(contentToDisplay, "\n")
	viewportHeight := m.height - 6 // Reserve space for title, borders, and help
	if viewportHeight < 5 {
		viewportHeight = 5
	}

	startLine := m.summaryViewport
	if startLine < 0 {
		startLine = 0
	}

	endLine := startLine + viewportHeight
	if endLine > len(lines) {
		endLine = len(lines)
	}

	// Display visible lines (trim trailing whitespace from glamour output)
	for i := startLine; i < endLine && i < len(lines); i++ {
		b.WriteString(strings.TrimRight(lines[i], " ") + "\n")
	}

	// Show scroll indicator if needed
	if endLine < len(lines) {
		b.WriteString(listHelpStyle.Render(fmt.Sprintf("  ... (%d more lines)", len(lines)-endLine)) + "\n")
	}

	// Help footer
	b.WriteString("\n" + listHelpStyle.Render("↑/↓/j/k: scroll  q/esc: back to skills list"))

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

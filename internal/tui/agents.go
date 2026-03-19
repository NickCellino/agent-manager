package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"

	"agent-manager/internal/agents"
	"agent-manager/internal/models"
	"agent-manager/internal/storage"
)

// AgentsModel manages the agents selection TUI
type AgentsModel struct {
	allAgents       []models.Agent
	filteredAgents  []models.Agent
	selectedAgents  map[string]bool
	installedAgents map[string]bool
	lockFile        *models.LockFile
	textInput       textinput.Model
	cursor          int
	filter          string
	mode            string // "select", "confirm-delete"
	inputMode       string // "navigate", "filter"
	agentsToDelete  []string
	err             error
	width           int
	height          int
	done            bool
	saved           bool
}

// NewAgentsModel creates a new agents selection model
func NewAgentsModel() (*AgentsModel, error) {
	allAgents, err := agents.DiscoverAgents()
	if err != nil {
		return nil, err
	}

	lockFile, err := storage.LoadLockFile()
	if err != nil {
		return nil, err
	}

	installed, err := agents.ListInstalledAgents()
	if err != nil {
		return nil, err
	}

	installedAgents := make(map[string]bool)
	for _, agentPath := range installed {
		installedAgents[agentPath] = true
	}

	// Build selected set from lock file
	selectedAgents := make(map[string]bool)
	for _, entry := range lockFile.Agents {
		key := fmt.Sprintf("%s|%s|%s", entry.Name, entry.Registry.Type, entry.Registry.Location)
		selectedAgents[key] = true
	}

	ti := textinput.New()
	ti.Placeholder = "type to filter..."
	ti.CharLimit = 156
	ti.Cursor.Style = lipgloss.NewStyle()
	ti.Prompt = ""

	return &AgentsModel{
		allAgents:       allAgents,
		filteredAgents:  allAgents,
		selectedAgents:  selectedAgents,
		installedAgents: installedAgents,
		lockFile:        lockFile,
		textInput:       ti,
		mode:            "select",
		inputMode:       "navigate",
	}, nil
}

func (m *AgentsModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *AgentsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.textInput.Width = msg.Width - 20

	case tea.KeyMsg:
		switch m.mode {
		case "select":
			if m.inputMode == "filter" {
				switch msg.Type {
				case tea.KeyEsc:
					m.inputMode = "navigate"
					m.textInput.Blur()
					return m, nil
				default:
					var cmd tea.Cmd
					oldValue := m.textInput.Value()
					m.textInput, cmd = m.textInput.Update(msg)
					if m.textInput.Value() != oldValue {
						m.filter = m.textInput.Value()
						m.applyFilter()
					}
					return m, cmd
				}
			} else {
				return m.updateNavigateMode(msg)
			}
		case "confirm-delete":
			return m.updateConfirmDeleteMode(msg)
		}
	}

	return m, nil
}

func (m *AgentsModel) updateNavigateMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		m.done = true
		return m, tea.Quit

	case tea.KeyEnter:
		if err := m.saveSelections(); err != nil {
			m.err = err
			return m, tea.Quit
		}

		if len(m.agentsToDelete) > 0 {
			m.mode = "confirm-delete"
			return m, nil
		}

		m.saved = true
		m.done = true
		return m, tea.Quit

	case tea.KeySpace:
		if len(m.filteredAgents) > 0 && m.cursor < len(m.filteredAgents) {
			agent := m.filteredAgents[m.cursor]
			agentKey := fmt.Sprintf("%s|%s|%s", agent.Name, agent.Registry.Type, agent.Registry.Location)
			if m.selectedAgents[agentKey] {
				delete(m.selectedAgents, agentKey)
			} else {
				m.selectedAgents[agentKey] = true
			}
		}
		return m, nil

	case tea.KeyUp, tea.KeyDown:
		if msg.Type == tea.KeyUp && m.cursor > 0 {
			m.cursor--
		} else if msg.Type == tea.KeyDown && m.cursor < len(m.filteredAgents)-1 {
			m.cursor++
		}
		return m, nil

	case tea.KeyEsc:
		m.done = true
		return m, tea.Quit

	case tea.KeyRunes:
		if len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case '/':
				m.inputMode = "filter"
				m.textInput.Focus()
				return m, textinput.Blink
			case 'k':
				if m.cursor > 0 {
					m.cursor--
				}
				return m, nil
			case 'j':
				if m.cursor < len(m.filteredAgents)-1 {
					m.cursor++
				}
				return m, nil
			}
		}
	}

	return m, nil
}

func (m *AgentsModel) updateConfirmDeleteMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		for _, agentPath := range m.agentsToDelete {
			entry := storage.GetManagedAgentEntry(m.lockFile, agentPath)
			if entry == nil {
				continue
			}
			if err := agents.RemoveAgentFromProject(entry, m.lockFile); err != nil {
				fmt.Printf("Warning: failed to remove %s: %v\n", entry.Name, err)
			}
		}

		m.saved = true
		m.done = true
		return m, tea.Quit

	case "n", "N", "esc":
		m.saved = true
		m.done = true
		return m, tea.Quit
	}

	return m, nil
}

func (m *AgentsModel) applyFilter() {
	if m.filter == "" {
		m.filteredAgents = m.allAgents
		m.cursor = 0
		return
	}

	var names []string
	for _, agent := range m.allAgents {
		names = append(names, agent.Name)
	}

	matches := fuzzy.Find(m.filter, names)
	m.filteredAgents = nil
	for _, match := range matches {
		for _, agent := range m.allAgents {
			if agent.Name == match.Str {
				m.filteredAgents = append(m.filteredAgents, agent)
				break
			}
		}
	}
	m.cursor = 0
}

func (m *AgentsModel) saveSelections() error {
	// Install newly selected agents
	for _, agent := range m.allAgents {
		agentKey := fmt.Sprintf("%s|%s|%s", agent.Name, agent.Registry.Type, agent.Registry.Location)
		if m.selectedAgents[agentKey] {
			if storage.FindAgentLockFileEntry(m.lockFile, agent.Name, agent.Registry) != nil {
				continue
			}
			if _, err := agents.AddAgentToProject(agent, m.lockFile); err != nil {
				return err
			}
		}
	}

	// Find agents to delete
	for agentPath := range m.installedAgents {
		entry := storage.GetManagedAgentEntry(m.lockFile, agentPath)
		if entry == nil {
			continue
		}

		agentKey := fmt.Sprintf("%s|%s|%s", entry.Name, entry.Registry.Type, entry.Registry.Location)
		if !m.selectedAgents[agentKey] {
			m.agentsToDelete = append(m.agentsToDelete, agentPath)
		}
	}

	return nil
}

func (m *AgentsModel) View() string {
	switch m.mode {
	case "select":
		return m.viewSelect()
	case "confirm-delete":
		return m.viewConfirmDelete()
	default:
		return ""
	}
}

func (m *AgentsModel) viewSelect() string {
	var b strings.Builder

	b.WriteString(listTitleStyle.Render("Select Agents") + "\n")

	if m.inputMode == "filter" {
		b.WriteString("Filter: " + m.textInput.View() + "\n")
	} else {
		if m.textInput.Value() == "" {
			b.WriteString("Filter: " + listHelpStyle.Render("press / to filter") + "\n")
		} else {
			b.WriteString("Filter: " + m.textInput.View() + "\n")
		}
	}

	if len(m.filteredAgents) == 0 {
		b.WriteString("  No agents found\n")
	} else {
		for i, agent := range m.filteredAgents {
			cursor := "  "
			if i == m.cursor {
				cursor = "> "
			}

			agentKey := fmt.Sprintf("%s|%s|%s", agent.Name, agent.Registry.Type, agent.Registry.Location)
			checked := "[ ]"
			if m.selectedAgents[agentKey] {
				checked = "[x]"
			}

			line := fmt.Sprintf("%s%s %s %s", cursor, checked, agent.Name, formatRegistryDisplay(agent.Registry))

			if i == m.cursor {
				b.WriteString(listSelectedItemStyle.Render(line) + "\n")
			} else {
				b.WriteString(listItemStyle.Render(line) + "\n")
			}
		}
	}

	b.WriteString(fmt.Sprintf("\n%d/%d agents selected\n", len(m.selectedAgents), len(m.allAgents)))

	if m.inputMode == "filter" {
		b.WriteString("\n" + listHelpStyle.Render("esc: exit filter  type to search"))
	} else {
		b.WriteString("\n" + listHelpStyle.Render("/: filter  ↑/↓/j/k: navigate  space: toggle  enter: save  esc: cancel"))
	}

	return b.String()
}

func (m *AgentsModel) viewConfirmDelete() string {
	var b strings.Builder

	b.WriteString(listTitleStyle.Render("Confirm Deletion") + "\n\n")
	b.WriteString("The following agents will be removed:\n\n")

	for _, agentName := range m.agentsToDelete {
		b.WriteString(fmt.Sprintf("  - %s\n", agentName))
	}

	b.WriteString("\n" + listHelpStyle.Render("y: confirm  n: cancel"))

	return b.String()
}

// RunAgentsTUI runs the agents selection TUI
func RunAgentsTUI() (bool, error) {
	model, err := NewAgentsModel()
	if err != nil {
		return false, err
	}

	p := tea.NewProgram(model)
	m, err := p.Run()
	if err != nil {
		return false, err
	}

	result := m.(*AgentsModel)

	if result.err != nil {
		return false, result.err
	}

	return result.saved, nil
}

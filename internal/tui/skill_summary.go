package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"agent-manager/internal/models"
)

// SkillSummaryModel displays a skill summary with markdown rendering
type SkillSummaryModel struct {
	skill    models.Skill
	summary  string
	viewport viewport.Model
	width    int
	height   int
	ready    bool
	err      error
}

// NewSkillSummaryModel creates a new skill summary model
func NewSkillSummaryModel(skill models.Skill, summary string) *SkillSummaryModel {
	return &SkillSummaryModel{
		skill:   skill,
		summary: summary,
	}
}

func (m *SkillSummaryModel) Init() tea.Cmd {
	return nil
}

func (m *SkillSummaryModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		if !m.ready {
			// Initialize viewport
			m.viewport = viewport.New(msg.Width, msg.Height-4)
			m.viewport.Style = lipgloss.NewStyle()

			// Render markdown content
			content, err := m.renderMarkdown()
			if err != nil {
				m.err = err
				content = m.summary // Fallback to plain text
			}

			m.viewport.SetContent(content)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 4

			// Re-render with new width
			content, err := m.renderMarkdown()
			if err != nil {
				m.err = err
				content = m.summary
			}
			m.viewport.SetContent(content)
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			// Return to skills list
			return m, tea.Quit
		}
	}

	// Update viewport
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *SkillSummaryModel) renderMarkdown() (string, error) {
	// Create glamour renderer with a style that works well in terminals
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(m.width-4),
	)
	if err != nil {
		return "", err
	}

	// Render the markdown content
	out, err := renderer.Render(m.summary)
	if err != nil {
		return "", err
	}

	return out, nil
}

func (m *SkillSummaryModel) View() string {
	if !m.ready {
		return "Loading..."
	}

	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress 'q' to go back", m.err)
	}

	var b strings.Builder

	// Title bar
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	b.WriteString(titleStyle.Render(fmt.Sprintf(" Skill: %s ", m.skill.Name)) + "\n")

	// Content viewport
	b.WriteString(m.viewport.View())

	// Help footer
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	b.WriteString("\n" + helpStyle.Render("↑/↓: scroll  q/esc: back to skills list"))

	return b.String()
}

// RunSkillSummaryTUI runs the skill summary TUI
func RunSkillSummaryTUI(skill models.Skill, summary string) error {
	model := NewSkillSummaryModel(skill, summary)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

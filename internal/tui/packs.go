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
	"agent-manager/internal/skills"
	"agent-manager/internal/storage"
)

// PacksModel manages the packs TUI
type PacksModel struct {
	store     *models.PackStore
	mode      string // "list", "add-name", "confirm-delete", "update-pack"
	textInput textinput.Model
	cursor    int
	selected  *models.Pack
	err       error
	width     int
	height    int

	// update-pack mode fields
	updatePack     models.Pack
	activeTab      int // 0 = skills, 1 = agents
	allSkills      []models.Skill
	filteredSkills []models.Skill
	allAgents      []models.Agent
	filteredAgents []models.Agent
	selectedSkills map[string]bool // key: "name|regType|regLoc"
	selectedAgents map[string]bool
	tabCursor      int    // cursor within current tab
	tabFilter      string // current filter text
	tabInputMode   string // "navigate" or "filter"
	tabTextInput   textinput.Model
}

// NewPacksModel creates a new packs model
func NewPacksModel() (*PacksModel, error) {
	store, err := storage.LoadPacks()
	if err != nil {
		return nil, err
	}

	ti := textinput.New()
	ti.Placeholder = "pack name..."
	ti.Focus()
	ti.CharLimit = 64

	tabTI := textinput.New()
	tabTI.Placeholder = "type to filter..."
	tabTI.CharLimit = 156
	tabTI.Cursor.Style = lipgloss.NewStyle()
	tabTI.Prompt = ""

	return &PacksModel{
		store:        store,
		mode:         "list",
		textInput:    ti,
		tabTextInput: tabTI,
		tabInputMode: "navigate",
	}, nil
}

func (m *PacksModel) Init() tea.Cmd {
	return nil
}

func (m *PacksModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.textInput.Width = msg.Width - 10
		m.tabTextInput.Width = msg.Width - 20

	case tea.KeyMsg:
		switch m.mode {
		case "list":
			return m.updateListMode(msg)
		case "add-name":
			return m.updateAddNameMode(msg)
		case "confirm-delete":
			return m.updateConfirmDeleteMode(msg)
		case "update-pack":
			return m.updatePackMode(msg)
		}
	}

	return m, nil
}

func (m *PacksModel) updateListMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "a":
		m.textInput.SetValue("")
		m.textInput.Focus()
		m.mode = "add-name"
		return m, textinput.Blink
	case "d":
		if len(m.store.Packs) > 0 && m.cursor < len(m.store.Packs) {
			p := m.store.Packs[m.cursor]
			m.selected = &p
			m.mode = "confirm-delete"
		}
		return m, nil
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case "down", "j":
		if m.cursor < len(m.store.Packs)-1 {
			m.cursor++
		}
		return m, nil
	case "enter":
		if len(m.store.Packs) > 0 && m.cursor < len(m.store.Packs) {
			return m.enterUpdateMode(m.store.Packs[m.cursor])
		}
		return m, nil
	}
	return m, nil
}

func (m *PacksModel) enterUpdateMode(pack models.Pack) (tea.Model, tea.Cmd) {
	// Discover skills and agents
	allSkills, err := skills.DiscoverSkills()
	if err != nil {
		m.err = err
		return m, tea.Quit
	}
	allAgents, err := agents.DiscoverAgents()
	if err != nil {
		m.err = err
		return m, tea.Quit
	}

	// Pre-select current pack items
	selectedSkills := make(map[string]bool)
	for _, item := range pack.Skills {
		key := fmt.Sprintf("%s|%s|%s", item.Name, item.Registry.Type, item.Registry.Location)
		selectedSkills[key] = true
	}
	selectedAgents := make(map[string]bool)
	for _, item := range pack.Agents {
		key := fmt.Sprintf("%s|%s|%s", item.Name, item.Registry.Type, item.Registry.Location)
		selectedAgents[key] = true
	}

	m.updatePack = pack
	m.allSkills = allSkills
	m.filteredSkills = allSkills
	m.allAgents = allAgents
	m.filteredAgents = allAgents
	m.selectedSkills = selectedSkills
	m.selectedAgents = selectedAgents
	m.activeTab = 0
	m.tabCursor = 0
	m.tabFilter = ""
	m.tabInputMode = "navigate"
	m.tabTextInput.SetValue("")
	m.tabTextInput.Blur()
	m.mode = "update-pack"
	return m, nil
}

func (m *PacksModel) updateAddNameMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		name := strings.TrimSpace(m.textInput.Value())
		if name == "" {
			return m, nil
		}
		newPack := models.Pack{
			Name:   name,
			Skills: []models.PackItem{},
			Agents: []models.PackItem{},
		}
		if err := storage.AddPack(m.store, newPack); err != nil {
			m.err = err
			return m, tea.Quit
		}
		// Place cursor on the new pack
		m.cursor = len(m.store.Packs) - 1
		m.mode = "list"
		return m, nil
	case tea.KeyEsc:
		m.mode = "list"
		return m, nil
	}
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m *PacksModel) updateConfirmDeleteMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		if m.selected != nil {
			if err := storage.RemovePack(m.store, m.selected.Name); err != nil {
				m.err = err
				return m, tea.Quit
			}
			if m.cursor >= len(m.store.Packs) && m.cursor > 0 {
				m.cursor--
			}
		}
		m.mode = "list"
		m.selected = nil
	case "n", "N", "esc":
		m.mode = "list"
		m.selected = nil
	}
	return m, nil
}

func (m *PacksModel) updatePackMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.tabInputMode == "filter" {
		switch msg.Type {
		case tea.KeyEsc:
			m.tabInputMode = "navigate"
			m.tabTextInput.Blur()
			return m, nil
		default:
			var cmd tea.Cmd
			old := m.tabTextInput.Value()
			m.tabTextInput, cmd = m.tabTextInput.Update(msg)
			if m.tabTextInput.Value() != old {
				m.tabFilter = m.tabTextInput.Value()
				m.applyTabFilter()
			}
			return m, cmd
		}
	}

	// navigate mode
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyEsc:
		m.mode = "list"
		return m, nil
	case tea.KeyEnter:
		if err := m.savePackUpdate(); err != nil {
			m.err = err
			return m, tea.Quit
		}
		m.mode = "list"
		return m, nil
	case tea.KeyTab:
		m.activeTab = 1 - m.activeTab
		m.tabCursor = 0
		m.tabFilter = ""
		m.tabTextInput.SetValue("")
		m.applyTabFilter()
		return m, nil
	case tea.KeySpace:
		m.toggleCurrentItem()
		return m, nil
	case tea.KeyUp:
		if m.tabCursor > 0 {
			m.tabCursor--
		}
		return m, nil
	case tea.KeyDown:
		maxIdx := m.currentTabLen() - 1
		if m.tabCursor < maxIdx {
			m.tabCursor++
		}
		return m, nil
	case tea.KeyRunes:
		if len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case '/':
				m.tabInputMode = "filter"
				m.tabTextInput.Focus()
				return m, textinput.Blink
			case 'k':
				if m.tabCursor > 0 {
					m.tabCursor--
				}
				return m, nil
			case 'j':
				maxIdx := m.currentTabLen() - 1
				if m.tabCursor < maxIdx {
					m.tabCursor++
				}
				return m, nil
			}
		}
	}
	return m, nil
}

func (m *PacksModel) currentTabLen() int {
	if m.activeTab == 0 {
		return len(m.filteredSkills)
	}
	return len(m.filteredAgents)
}

func (m *PacksModel) toggleCurrentItem() {
	if m.activeTab == 0 {
		if m.tabCursor < len(m.filteredSkills) {
			s := m.filteredSkills[m.tabCursor]
			key := fmt.Sprintf("%s|%s|%s", s.Name, s.Registry.Type, s.Registry.Location)
			if m.selectedSkills[key] {
				delete(m.selectedSkills, key)
			} else {
				m.selectedSkills[key] = true
			}
		}
	} else {
		if m.tabCursor < len(m.filteredAgents) {
			a := m.filteredAgents[m.tabCursor]
			key := fmt.Sprintf("%s|%s|%s", a.Name, a.Registry.Type, a.Registry.Location)
			if m.selectedAgents[key] {
				delete(m.selectedAgents, key)
			} else {
				m.selectedAgents[key] = true
			}
		}
	}
}

func (m *PacksModel) applyTabFilter() {
	if m.activeTab == 0 {
		if m.tabFilter == "" {
			m.filteredSkills = m.allSkills
			m.tabCursor = 0
			return
		}
		// Build name→skill map for O(n+m) lookup
		skillByName := make(map[string]models.Skill, len(m.allSkills))
		names := make([]string, len(m.allSkills))
		for i, s := range m.allSkills {
			names[i] = s.Name
			skillByName[s.Name] = s
		}
		matches := fuzzy.Find(m.tabFilter, names)
		m.filteredSkills = make([]models.Skill, 0, len(matches))
		for _, match := range matches {
			if s, ok := skillByName[match.Str]; ok {
				m.filteredSkills = append(m.filteredSkills, s)
			}
		}
		m.tabCursor = 0
	} else {
		if m.tabFilter == "" {
			m.filteredAgents = m.allAgents
			m.tabCursor = 0
			return
		}
		// Build name→agent map for O(n+m) lookup
		agentByName := make(map[string]models.Agent, len(m.allAgents))
		names := make([]string, len(m.allAgents))
		for i, a := range m.allAgents {
			names[i] = a.Name
			agentByName[a.Name] = a
		}
		matches := fuzzy.Find(m.tabFilter, names)
		m.filteredAgents = make([]models.Agent, 0, len(matches))
		for _, match := range matches {
			if a, ok := agentByName[match.Str]; ok {
				m.filteredAgents = append(m.filteredAgents, a)
			}
		}
		m.tabCursor = 0
	}
}

func (m *PacksModel) savePackUpdate() error {
	// Build skill items from selected keys
	var packSkills []models.PackItem
	for _, s := range m.allSkills {
		key := fmt.Sprintf("%s|%s|%s", s.Name, s.Registry.Type, s.Registry.Location)
		if m.selectedSkills[key] {
			packSkills = append(packSkills, models.PackItem{
				Name:     s.Name,
				Registry: s.Registry,
			})
		}
	}
	if packSkills == nil {
		packSkills = []models.PackItem{}
	}

	// Build agent items from selected keys
	var packAgents []models.PackItem
	for _, a := range m.allAgents {
		key := fmt.Sprintf("%s|%s|%s", a.Name, a.Registry.Type, a.Registry.Location)
		if m.selectedAgents[key] {
			packAgents = append(packAgents, models.PackItem{
				Name:     a.Name,
				Registry: a.Registry,
			})
		}
	}
	if packAgents == nil {
		packAgents = []models.PackItem{}
	}

	m.updatePack.Skills = packSkills
	m.updatePack.Agents = packAgents
	return storage.UpdatePack(m.store, m.updatePack)
}

func (m *PacksModel) View() string {
	switch m.mode {
	case "list":
		return m.viewList()
	case "add-name":
		return m.viewAddName()
	case "confirm-delete":
		return m.viewConfirmDelete()
	case "update-pack":
		return m.viewUpdatePack()
	default:
		return ""
	}
}

func (m *PacksModel) viewList() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Packs") + "\n\n")

	if len(m.store.Packs) == 0 {
		b.WriteString("  No packs configured.\n\n")
	} else {
		for i, p := range m.store.Packs {
			line := fmt.Sprintf("%s  (%d skills, %d agents)", p.Name, len(p.Skills), len(p.Agents))
			if i == m.cursor {
				b.WriteString(selectedItemStyle.Render("> "+line) + "\n")
			} else {
				b.WriteString(itemStyle.Render("  "+line) + "\n")
			}
		}
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("a: add  d: delete  enter: update  ↑/↓/j/k: navigate  q: quit"))
	return b.String()
}

func (m *PacksModel) viewAddName() string {
	return titleStyle.Render("Add Pack") + "\n\n" +
		"Enter pack name:\n\n" +
		m.textInput.View() + "\n\n" +
		helpStyle.Render("enter: save  esc: cancel")
}

func (m *PacksModel) viewConfirmDelete() string {
	name := ""
	if m.selected != nil {
		name = m.selected.Name
	}
	return titleStyle.Render("Confirm Delete") + "\n\n" +
		fmt.Sprintf("Are you sure you want to delete pack %q?\n\n", name) +
		helpStyle.Render("y: yes  n: no")
}

func (m *PacksModel) viewUpdatePack() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(fmt.Sprintf("Update Pack: %s", m.updatePack.Name)) + "\n\n")

	// Tab headers
	skillsTab := "  Skills  "
	agentsTab := "  Agents  "
	tabStyle := lipgloss.NewStyle().Padding(0, 1)
	activeTabStyle := lipgloss.NewStyle().Padding(0, 1).Bold(true).Foreground(lipgloss.Color("170"))
	separatorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	if m.activeTab == 0 {
		b.WriteString(activeTabStyle.Render(skillsTab))
		b.WriteString(tabStyle.Render(agentsTab))
	} else {
		b.WriteString(tabStyle.Render(skillsTab))
		b.WriteString(activeTabStyle.Render(agentsTab))
	}
	// Horizontal separator line
	b.WriteString("\n" + separatorStyle.Render(strings.Repeat("─", 40)) + "\n")

	// Filter line
	if m.tabInputMode == "filter" {
		b.WriteString("Filter: " + m.tabTextInput.View() + "\n")
	} else {
		if m.tabTextInput.Value() == "" {
			b.WriteString("Filter: " + listHelpStyle.Render("press / to filter") + "\n")
		} else {
			b.WriteString("Filter: " + m.tabTextInput.View() + "\n")
		}
	}

	// Item list
	if m.activeTab == 0 {
		if len(m.filteredSkills) == 0 {
			b.WriteString("  No skills found\n")
		} else {
			for i, s := range m.filteredSkills {
				key := fmt.Sprintf("%s|%s|%s", s.Name, s.Registry.Type, s.Registry.Location)
				cursor := "  "
				if i == m.tabCursor {
					cursor = "> "
				}
				checked := "[ ]"
				if m.selectedSkills[key] {
					checked = "[x]"
				}
				line := fmt.Sprintf("%s%s %s %s", cursor, checked, s.Name, formatRegistryDisplay(s.Registry))
				if i == m.tabCursor {
					b.WriteString(listSelectedItemStyle.Render(line) + "\n")
				} else {
					b.WriteString(listItemStyle.Render(line) + "\n")
				}
			}
		}
		b.WriteString(fmt.Sprintf("\n%d/%d skills selected\n", len(m.selectedSkills), len(m.allSkills)))
	} else {
		if len(m.filteredAgents) == 0 {
			b.WriteString("  No agents found\n")
		} else {
			for i, a := range m.filteredAgents {
				key := fmt.Sprintf("%s|%s|%s", a.Name, a.Registry.Type, a.Registry.Location)
				cursor := "  "
				if i == m.tabCursor {
					cursor = "> "
				}
				checked := "[ ]"
				if m.selectedAgents[key] {
					checked = "[x]"
				}
				line := fmt.Sprintf("%s%s %s %s", cursor, checked, a.Name, formatRegistryDisplay(a.Registry))
				if i == m.tabCursor {
					b.WriteString(listSelectedItemStyle.Render(line) + "\n")
				} else {
					b.WriteString(listItemStyle.Render(line) + "\n")
				}
			}
		}
		b.WriteString(fmt.Sprintf("\n%d/%d agents selected\n", len(m.selectedAgents), len(m.allAgents)))
	}

	// Help
	if m.tabInputMode == "filter" {
		b.WriteString("\n" + listHelpStyle.Render("esc: exit filter  type to search"))
	} else {
		b.WriteString("\n" + listHelpStyle.Render("tab: switch tab  /: filter  ↑/↓/j/k: navigate  space: toggle  enter: save  esc: cancel"))
	}

	return b.String()
}

// RunPacksTUI runs the packs management TUI
func RunPacksTUI() error {
	model, err := NewPacksModel()
	if err != nil {
		return err
	}

	p := tea.NewProgram(model)
	m, err := p.Run()
	if err != nil {
		return err
	}

	return m.(*PacksModel).err
}

// RunPackUpdateTUI opens the update TUI for a specific pack by name
func RunPackUpdateTUI(name string) error {
	store, err := storage.LoadPacks()
	if err != nil {
		return err
	}

	pack := storage.FindPack(store, name)
	if pack == nil {
		return fmt.Errorf("pack %q not found", name)
	}

	model, err := NewPacksModel()
	if err != nil {
		return err
	}

	// Switch directly to update mode for this pack
	_, cmd := model.enterUpdateMode(*pack)
	_ = cmd // initial cmd not needed before Run

	p := tea.NewProgram(model)
	m, err := p.Run()
	if err != nil {
		return err
	}

	return m.(*PacksModel).err
}

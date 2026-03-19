package tui

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"agent-manager/internal/models"
	"agent-manager/internal/storage"
)

// RegistryModel manages the registry TUI
type RegistryModel struct {
	store             *models.RegistryStore
	list              list.Model
	mode              string // "list", "add-type", "add-location", "confirm-delete"
	textInput         textinput.Model
	newRegistry       models.Registry
	selected          *models.Registry
	err               error
	width             int
	height            int
	registryTypeIndex int // 0 = GitHub, 1 = Local (for add-type mode)
}

// Registry types for selection
var registryTypes = []struct {
	name string
	typ  models.RegistryType
	desc string
}{
	{"GitHub repository", models.RegistryTypeGitHub, "owner/repo format"},
	{"Local directory", models.RegistryTypeLocal, "absolute or ~/path"},
}

// RegistryItem represents a registry in the list
type RegistryItem struct {
	registry models.Registry
}

func (i RegistryItem) FilterValue() string { return i.registry.Location }
func (i RegistryItem) Title() string       { return string(i.registry.Type) }
func (i RegistryItem) Description() string {
	return i.registry.Location
}

type registryDelegate struct{}

func (d registryDelegate) Height() int                             { return 1 }
func (d registryDelegate) Spacing() int                            { return 0 }
func (d registryDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d registryDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(RegistryItem)
	if !ok {
		return
	}

	str := fmt.Sprintf("%s\n  %s", i.Title(), i.Description())

	fn := itemStyle.Render
	if index == m.Index() {
		fn = func(s ...string) string {
			return selectedItemStyle.Render("> " + strings.Join(s, " "))
		}
	}

	fmt.Fprint(w, fn(str))
}

var (
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
	helpStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

// NewRegistryModel creates a new registry model
func NewRegistryModel() (*RegistryModel, error) {
	store, err := storage.LoadRegistries()
	if err != nil {
		return nil, err
	}

	items := make([]list.Item, len(store.Registries))
	for i, r := range store.Registries {
		items[i] = RegistryItem{registry: r}
	}

	l := list.New(items, registryDelegate{}, 0, 0)
	l.Title = "Skill Registries"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle
	l.Styles.HelpStyle = helpStyle

	ti := textinput.New()
	ti.Placeholder = ""
	ti.Focus()
	ti.CharLimit = 156

	return &RegistryModel{
		store:     store,
		list:      l,
		mode:      "list",
		textInput: ti,
	}, nil
}

var titleStyle = lipgloss.NewStyle().
	MarginLeft(2).
	MarginBottom(1).
	Bold(true).
	Foreground(lipgloss.Color("#FAFAFA")).
	Background(lipgloss.Color("#7D56F4")).
	Padding(0, 1)

func (m *RegistryModel) Init() tea.Cmd {
	return nil
}

func (m *RegistryModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height-4)
		m.textInput.Width = msg.Width - 10

	case tea.KeyMsg:
		switch m.mode {
		case "list":
			return m.updateListMode(msg)
		case "add-type":
			return m.updateAddTypeMode(msg)
		case "add-location":
			return m.updateAddLocationMode(msg)
		case "confirm-delete":
			return m.updateConfirmDeleteMode(msg)
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	m.textInput, _ = m.textInput.Update(msg)
	return m, cmd
}

func (m *RegistryModel) updateListMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "a":
		m.mode = "add-type"
		return m, nil
	case "d":
		if item, ok := m.list.SelectedItem().(RegistryItem); ok {
			m.selected = &item.registry
			m.mode = "confirm-delete"
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *RegistryModel) updateAddTypeMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.mode = "list"
		return m, nil
	case tea.KeyUp:
		if m.registryTypeIndex > 0 {
			m.registryTypeIndex--
		}
		return m, nil
	case tea.KeyDown:
		if m.registryTypeIndex < len(registryTypes)-1 {
			m.registryTypeIndex++
		}
		return m, nil
	case tea.KeyEnter:
		selectedType := registryTypes[m.registryTypeIndex]
		m.newRegistry.Type = selectedType.typ
		m.mode = "add-location"
		if m.newRegistry.Type == models.RegistryTypeGitHub {
			m.textInput.Placeholder = "GitHub location (e.g., owner/repo)"
		} else {
			m.textInput.Placeholder = "Local path (e.g., ~/Code/skills or /absolute/path)"
		}
		m.textInput.SetValue("")
		return m, nil
	case tea.KeyRunes:
		// Vim-style navigation (j/k)
		switch string(msg.Runes) {
		case "k":
			if m.registryTypeIndex > 0 {
				m.registryTypeIndex--
			}
			return m, nil
		case "j":
			if m.registryTypeIndex < len(registryTypes)-1 {
				m.registryTypeIndex++
			}
			return m, nil
		}
	}
	return m, nil
}

func (m *RegistryModel) updateAddLocationMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		location := strings.TrimSpace(m.textInput.Value())
		if location == "" {
			return m, nil
		}
		m.newRegistry.Location = location

		// Add the registry
		if err := storage.AddRegistry(m.store, m.newRegistry); err != nil {
			m.err = err
			return m, tea.Quit
		}

		// Refresh the list
		items := make([]list.Item, len(m.store.Registries))
		for i, r := range m.store.Registries {
			items[i] = RegistryItem{registry: r}
		}
		m.list.SetItems(items)

		m.mode = "list"
		m.newRegistry = models.Registry{}
		return m, nil
	case tea.KeyEsc:
		m.mode = "list"
		return m, nil
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m *RegistryModel) updateConfirmDeleteMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		if m.selected != nil {
			if err := storage.RemoveRegistry(m.store, m.selected.Type, m.selected.Location); err != nil {
				m.err = err
				return m, tea.Quit
			}

			// Refresh the list
			items := make([]list.Item, len(m.store.Registries))
			for i, r := range m.store.Registries {
				items[i] = RegistryItem{registry: r}
			}
			m.list.SetItems(items)
		}
		m.mode = "list"
		m.selected = nil
		return m, nil
	case "n", "N", "esc":
		m.mode = "list"
		m.selected = nil
		return m, nil
	}
	return m, nil
}

func (m *RegistryModel) View() string {
	switch m.mode {
	case "list":
		return m.viewList()
	case "add-type":
		return m.viewAddType()
	case "add-location":
		return m.viewAddLocation()
	case "confirm-delete":
		return m.viewConfirmDelete()
	default:
		return ""
	}
}

func (m *RegistryModel) viewList() string {
	help := helpStyle.Render("a: add  d: delete  q: quit")
	return m.list.View() + "\n" + help
}

func (m *RegistryModel) viewAddType() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Add Registry") + "\n\n")
	b.WriteString("Select registry type:\n\n")

	// Define base padding style for menu items
	menuBaseStyle := lipgloss.NewStyle().PaddingLeft(0)
	menuSelectedStyle := lipgloss.NewStyle().PaddingLeft(0).Foreground(lipgloss.Color("170"))

	for i, rt := range registryTypes {
		line := fmt.Sprintf("%s (%s)", rt.name, rt.desc)
		if i == m.registryTypeIndex {
			// Selected: cursor + highlighted text
			b.WriteString(menuSelectedStyle.Render("> " + line))
		} else {
			// Unselected: spaces + normal text
			b.WriteString(menuBaseStyle.Render("  " + line))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n" + helpStyle.Render("↑/↓/j/k: navigate  enter: select  esc: cancel"))

	return b.String()
}

func (m *RegistryModel) viewAddLocation() string {
	return titleStyle.Render("Add Registry") + "\n\n" +
		"Enter " + string(m.newRegistry.Type) + " location:\n\n" +
		m.textInput.View() + "\n\n" +
		helpStyle.Render("enter: save  esc: cancel")
}

func (m *RegistryModel) viewConfirmDelete() string {
	return titleStyle.Render("Confirm Delete") + "\n\n" +
		fmt.Sprintf("Are you sure you want to delete '%s' (%s)?\n\n", m.selected.Location, m.selected.Type) +
		helpStyle.Render("y: yes  n: no")
}

// RunRegistryTUI runs the registry management TUI
func RunRegistryTUI() error {
	model, err := NewRegistryModel()
	if err != nil {
		return err
	}

	p := tea.NewProgram(model)
	m, err := p.Run()
	if err != nil {
		return err
	}

	if m.(*RegistryModel).err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", m.(*RegistryModel).err)
		os.Exit(1)
	}

	return nil
}

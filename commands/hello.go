package commands

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/urfave/cli/v2"
)

type helloModel struct {
	textInput textinput.Model
	name      string
	quitting  bool
}

func initialHelloModel() helloModel {
	ti := textinput.New()
	ti.Placeholder = "Your name"
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 20

	return helloModel{
		textInput: ti,
	}
}

func (m helloModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m helloModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			m.name = m.textInput.Value()
			m.quitting = true
			return m, tea.Quit
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quitting = true
			return m, tea.Quit
		}
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m helloModel) View() string {
	if m.quitting {
		if m.name != "" {
			return fmt.Sprintf("Hello, %s!\n\n", m.name)
		}
		return "Goodbye!\n"
	}

	return fmt.Sprintf(
		"What's your name?\n\n%s\n\n(press enter to submit, esc to quit)\n",
		m.textInput.View(),
	)
}

func HelloCommand() *cli.Command {
	return &cli.Command{
		Name:  "hello",
		Usage: "Ask for your name and say hello",
		Action: func(c *cli.Context) error {
			p := tea.NewProgram(initialHelloModel())
			if _, err := p.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
				os.Exit(1)
			}
			return nil
		},
	}
}

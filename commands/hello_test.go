package commands

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestHelloModel_Update(t *testing.T) {
	m := initialHelloModel()

	// Simulate typing a name
	m.textInput.SetValue("Alice")

	// Simulate pressing enter
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	updatedModel, ok := newModel.(helloModel)
	if !ok {
		t.Fatal("Expected helloModel type")
	}

	if updatedModel.name != "Alice" {
		t.Errorf("Expected name 'Alice', got '%s'", updatedModel.name)
	}

	if !updatedModel.quitting {
		t.Error("Expected quitting to be true")
	}

	if cmd == nil {
		t.Error("Expected quit command")
	}
}

func TestHelloModel_View(t *testing.T) {
	m := initialHelloModel()
	view := m.View()

	if view == "" {
		t.Error("Expected non-empty view")
	}

	// After quitting with a name
	m.name = "Bob"
	m.quitting = true
	view = m.View()

	if view != "Hello, Bob!\n\n" {
		t.Errorf("Expected greeting message, got: %s", view)
	}
}

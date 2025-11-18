package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type TextInputModel struct {
	textInput textinput.Model
	label     string
	value     string
	submitted bool
}

func NewTextInput(label, placeholder string) TextInputModel {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 50
	ti.PromptStyle = FocusedStyle
	ti.TextStyle = lipgloss.NewStyle()

	return TextInputModel{
		textInput: ti,
		label:     label,
	}
}

func (m TextInputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m TextInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			m.value = m.textInput.Value()
			m.submitted = true
			return m, tea.Quit
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		}
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m TextInputModel) View() string {
	if m.submitted {
		return SuccessStyle.Render("✓ ") + m.label + ": " + m.value + "\n"
	}

	return fmt.Sprintf(
		"%s\n\n%s\n\n%s",
		FocusedStyle.Render(m.label),
		m.textInput.View(),
		HelpStyle.Render("enter: submit • esc: cancel"),
	)
}

func (m TextInputModel) Value() string {
	return m.value
}

// RunTextInput is a helper function to run the input
func RunTextInput(label, placeholder string) (string, error) {
	p := tea.NewProgram(NewTextInput(label, placeholder))
	m, err := p.Run()
	if err != nil {
		return "", err
	}

	if model, ok := m.(TextInputModel); ok {
		if !model.Submitted() {
			return "", fmt.Errorf("input cancelled")
		}
		return model.Value(), nil
	}

	return "", fmt.Errorf("unexpected model type")
}

func (m TextInputModel) Submitted() bool {
	return m.submitted
}
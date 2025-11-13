package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ConfirmModel struct {
	message  string
	result   bool
	answered bool
}

func NewConfirm(message string) ConfirmModel {
	return ConfirmModel{
		message: message,
	}
}

func (m ConfirmModel) Init() tea.Cmd {
	return nil
}

func (m ConfirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y":
			m.result = true
			m.answered = true
			return m, tea.Quit
		case "n", "N", "ctrl+c", "esc":
			m.result = false
			m.answered = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m ConfirmModel) View() string {
	if m.answered {
		if m.result {
			return SuccessStyle.Render("✓ Yes") + "\n"
		}
		return ErrorStyle.Render("✗ No") + "\n"
	}

	question := lipgloss.NewStyle().
		Bold(true).
		Foreground(PrimaryColor).
		Render(m.message)

	help := HelpStyle.Render("y: yes • n: no")

	return fmt.Sprintf("%s\n\n%s\n", question, help)
}

func (m ConfirmModel) Result() bool {
	return m.result
}

// RunConfirm é uma função helper
func RunConfirm(message string) (bool, error) {
	p := tea.NewProgram(NewConfirm(message))
	m, err := p.Run()
	if err != nil {
		return false, err
	}

	if model, ok := m.(ConfirmModel); ok {
		return model.Result(), nil
	}

	return false, fmt.Errorf("unexpected model type")
}

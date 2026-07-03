package tui

import (
	"fmt"
	"strconv"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type NumberInputModel struct {
	textInput textinput.Model
	label     string
	value     int64
	err       error
	submitted bool
}

func NewNumberInput(label, placeholder string) NumberInputModel {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Focus()
	ti.CharLimit = 20
	ti.Width = 50
	ti.PromptStyle = FocusedStyle
	ti.TextStyle = lipgloss.NewStyle()

	return NumberInputModel{
		textInput: ti,
		label:     label,
	}
}

func (m NumberInputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m NumberInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			val := m.textInput.Value()
			if val == "" {
				m.submitted = true
				return m, tea.Quit
			}
			
			num, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				m.err = err
				return m, nil
			}
			m.value = num
			m.submitted = true
			return m, tea.Quit
			
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		}
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m NumberInputModel) View() string {
	if m.submitted {
		return SuccessStyle.Render("✓ ") + m.label + ": " + fmt.Sprintf("%d", m.value) + "\n"
	}

	view := fmt.Sprintf(
		"%s\n\n%s\n\n",
		FocusedStyle.Render(m.label),
		m.textInput.View(),
	)
	
	if m.err != nil {
		view += ErrorStyle.Render("Invalid number format") + "\n"
	}
	
	view += HelpStyle.Render("enter: submit (empty to skip) • esc: cancel")
	
	return view
}

func (m NumberInputModel) Value() int64 {
	return m.value
}

// RunNumberInput is a helper function to run the number input
func RunNumberInput(label, placeholder string) (int64, error) {
	p := tea.NewProgram(NewNumberInput(label, placeholder))
	m, err := p.Run()
	if err != nil {
		return 0, err
	}

	if model, ok := m.(NumberInputModel); ok {
		return model.Value(), nil
	}

	return 0, fmt.Errorf("unexpected model type")
}

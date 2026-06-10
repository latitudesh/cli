package tui

import (
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ResourceDetailsModel renders one resource as a label/value sheet, like
// the server details view but with a caller-provided field order.
type ResourceDetailsModel struct {
	title    string
	fields   map[string]string
	order    []string
	quitting bool
}

func NewResourceDetails(title string, fields map[string]string, order []string) ResourceDetailsModel {
	return ResourceDetailsModel{
		title:  title,
		fields: fields,
		order:  order,
	}
}

func (m ResourceDetailsModel) Init() tea.Cmd {
	return nil
}

func (m ResourceDetailsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c", "backspace":
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m ResourceDetailsModel) View() string {
	if m.quitting {
		return ""
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(PrimaryColor).
		MarginBottom(1).
		Padding(0, 1)

	fieldLabelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(PrimaryColor).
		Width(20).
		Align(lipgloss.Right)

	fieldValueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("255")).
		Padding(0, 1)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(PrimaryColor).
		Padding(1, 2).
		MarginTop(1)

	renderField := func(key, value string) string {
		return lipgloss.JoinHorizontal(
			lipgloss.Top,
			fieldLabelStyle.Render(key+":"),
			fieldValueStyle.Render(value),
		)
	}

	var fields []string
	ordered := make(map[string]bool)
	for _, key := range m.order {
		ordered[key] = true
		if value, exists := m.fields[key]; exists && value != "" {
			fields = append(fields, renderField(key, value))
		}
	}

	// Render remaining fields alphabetically so the output is stable.
	var remaining []string
	for key, value := range m.fields {
		if !ordered[key] && value != "" {
			remaining = append(remaining, key)
		}
	}
	sort.Strings(remaining)
	for _, key := range remaining {
		fields = append(fields, renderField(key, m.fields[key]))
	}

	content := strings.Join(fields, "\n")

	box := boxStyle.Render(content)

	help := HelpStyle.Render("esc/backspace: back • q: quit")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render(m.title),
		box,
		"\n",
		help,
	)
}

// RunResourceDetails shows the details of a single resource.
func RunResourceDetails(title string, fields map[string]string, order []string) error {
	p := tea.NewProgram(
		NewResourceDetails(title, fields, order),
		tea.WithAltScreen(),
	)

	_, err := p.Run()
	return err
}

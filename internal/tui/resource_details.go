package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ResourceDetailsModel renders one resource as a label/value sheet, like
// the server details view but with a caller-provided field order. The sheet
// body lives in a viewport so long values (e.g. decoded cloud-init scripts)
// can be scrolled instead of overflowing the screen.
type ResourceDetailsModel struct {
	title    string
	fields   map[string]string
	order    []string
	viewport viewport.Model
	ready    bool
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
	case tea.WindowSizeMsg:
		headerHeight := lipgloss.Height(m.headerView())
		footerHeight := lipgloss.Height(m.footerView())
		bodyHeight := msg.Height - headerHeight - footerHeight
		if bodyHeight < 1 {
			bodyHeight = 1
		}

		if !m.ready {
			m.viewport = viewport.New(msg.Width, bodyHeight)
			m.viewport.SetHorizontalStep(8)
			m.viewport.SetContent(m.contentView())
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = bodyHeight
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m ResourceDetailsModel) headerView() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(PrimaryColor).
		MarginBottom(1).
		Padding(0, 1)

	return titleStyle.Render(m.title)
}

func (m ResourceDetailsModel) footerView() string {
	scroll := ""
	if m.ready && m.viewport.TotalLineCount() > m.viewport.Height {
		scroll = fmt.Sprintf(" • %3.0f%%", m.viewport.ScrollPercent()*100)
	}
	if m.ready && m.viewport.HorizontalScrollPercent() > 0 {
		scroll += fmt.Sprintf(" • →%3.0f%%", m.viewport.HorizontalScrollPercent()*100)
	}
	return HelpStyle.Render("↑/↓ ←/→: scroll • esc/backspace: back • q: quit" + scroll)
}

// contentView renders the label/value sheet that fills the viewport.
func (m ResourceDetailsModel) contentView() string {
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

	return boxStyle.Render(content)
}

func (m ResourceDetailsModel) View() string {
	if m.quitting {
		return ""
	}
	if !m.ready {
		return m.headerView()
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.headerView(),
		m.viewport.View(),
		m.footerView(),
	)
}

// RunResourceDetails shows the details of a single resource. Mouse cell
// motion is enabled so the wheel scrolls the details viewport.
func RunResourceDetails(title string, fields map[string]string, order []string) error {
	p := tea.NewProgram(
		NewResourceDetails(title, fields, order),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	_, err := p.Run()
	return err
}

package tui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type item struct {
	title, desc string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

// compactDelegate renders each item on a single line ("title  desc"),
// with no blank line between items, so many more entries are visible than
// with the default two-line delegate. The selected row is highlighted and
// the description is dimmed.
type compactDelegate struct{}

func (compactDelegate) Height() int                         { return 1 }
func (compactDelegate) Spacing() int                        { return 0 }
func (compactDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (compactDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	it, ok := listItem.(item)
	if !ok {
		return
	}

	var line string
	if index == m.Index() {
		s := "❯ " + it.title
		if it.desc != "" {
			s += "  " + it.desc
		}
		line = FocusedStyle.Render(s)
	} else {
		line = "  " + it.title
		if it.desc != "" {
			line += "  " + lipgloss.NewStyle().Foreground(MutedColor).Render(it.desc)
		}
	}

	// Keep every item exactly one line so the list height stays correct.
	fmt.Fprint(w, lipgloss.NewStyle().MaxWidth(m.Width()).Render(line))
}

type ListModel struct {
	list     list.Model
	choice   string
	quitting bool
}

func NewList(title string, items []string, descriptions []string) ListModel {
	listItems := make([]list.Item, len(items))
	for i, itemStr := range items {
		desc := ""
		if i < len(descriptions) {
			desc = descriptions[i]
		}
		listItems[i] = item{title: itemStr, desc: desc}
	}

	const defaultWidth = 80
	const listHeight = 14

	l := list.New(listItems, compactDelegate{}, defaultWidth, listHeight)
	l.Title = title
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = TitleStyle
	l.Styles.PaginationStyle = lipgloss.NewStyle().Foreground(MutedColor)
	l.Styles.HelpStyle = HelpStyle

	return ListModel{
		list: l,
	}
}

func (m ListModel) Init() tea.Cmd {
	return nil
}

func (m ListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		// Use the available terminal height (minus room for the title and
		// help/pagination footer) so as many items as possible are visible.
		if h := msg.Height - 6; h > 4 {
			m.list.SetHeight(h)
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit

		case "enter":
			i, ok := m.list.SelectedItem().(item)
			if ok {
				m.choice = i.title
			}
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m ListModel) View() string {
	if m.choice != "" {
		return SuccessStyle.Render("✓ Selected: ") + m.choice + "\n"
	}
	if m.quitting {
		return "Cancelled.\n"
	}
	return "\n" + m.list.View()
}

func (m ListModel) Choice() string {
	return m.choice
}

// RunList é uma função helper para executar a lista
func RunList(title string, items []string, descriptions []string) (string, error) {
	p := tea.NewProgram(NewList(title, items, descriptions))
	m, err := p.Run()
	if err != nil {
		return "", err
	}

	if model, ok := m.(ListModel); ok {
		if model.Choice() == "" {
			return "", fmt.Errorf("selection cancelled")
		}
		return model.Choice(), nil
	}

	return "", fmt.Errorf("unexpected model type")
}

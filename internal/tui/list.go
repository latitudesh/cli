package tui

import (
	"fmt"

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

	l := list.New(listItems, list.NewDefaultDelegate(), defaultWidth, listHeight)
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

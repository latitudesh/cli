package tui

import (
	"errors"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// MultiSelectModel is a compact checkbox list: ↑/↓ (or k/j) move, space toggles,
// enter confirms, a toggles all, esc/ctrl+c cancels. It mirrors the look of the
// single-select list (TitleStyle, SelectedStyle, HelpStyle) so multi-selection
// feels like the rest of the CLI.
type MultiSelectModel struct {
	title    string
	items    []string
	descs    []string
	cursor   int
	selected map[int]bool
	done     bool
	canceled bool
}

// NewMultiSelect builds the model. Descriptions are optional (may be shorter
// than items).
func NewMultiSelect(title string, items, descriptions []string) MultiSelectModel {
	return MultiSelectModel{
		title:    title,
		items:    items,
		descs:    descriptions,
		selected: make(map[int]bool),
	}
}

func (m MultiSelectModel) Init() tea.Cmd { return nil }

func (m MultiSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "ctrl+c", "esc", "q":
			m.canceled = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case " ", "x":
			m.selected[m.cursor] = !m.selected[m.cursor]
		case "a":
			// Count the entries that are actually on: toggling an item off
			// leaves a false value behind, so len(m.selected) overcounts.
			on := 0
			for i := range m.items {
				if m.selected[i] {
					on++
				}
			}
			all := on < len(m.items)
			m.selected = make(map[int]bool, len(m.items))
			if all {
				for i := range m.items {
					m.selected[i] = true
				}
			}
		case "enter":
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m MultiSelectModel) View() string {
	if m.done || m.canceled {
		return ""
	}
	var b strings.Builder
	b.WriteString(TitleStyle.Render(m.title) + "\n\n")
	for i, it := range m.items {
		cursor := "  "
		if i == m.cursor {
			cursor = SelectedStyle.Render("> ")
		}
		box := "[ ]"
		if m.selected[i] {
			box = SelectedStyle.Render("[x]")
		}
		line := fmt.Sprintf("%s%s %s", cursor, box, it)
		if i == m.cursor {
			line = SelectedStyle.Render(line)
		}
		b.WriteString(line + "\n")
		if i == m.cursor && i < len(m.descs) && m.descs[i] != "" {
			b.WriteString("      " + lipgloss.NewStyle().Foreground(MutedColor).Render(m.descs[i]) + "\n")
		}
	}
	b.WriteString("\n" + HelpStyle.Render("↑/↓: move • space: toggle • a: all • enter: confirm • esc: cancel") + "\n")
	return b.String()
}

// Selected returns the chosen indices in ascending order.
func (m MultiSelectModel) Selected() []int {
	out := make([]int, 0, len(m.selected))
	for i := 0; i < len(m.items); i++ {
		if m.selected[i] {
			out = append(out, i)
		}
	}
	return out
}

// Canceled reports whether the user aborted the selection.
func (m MultiSelectModel) Canceled() bool { return m.canceled }

// ErrCanceled is returned when the user aborts a prompt (esc/ctrl+c), so
// callers can exit "refused" instead of reporting a usage error.
var ErrCanceled = errors.New("selection cancelled")

// RunMultiSelect shows the checkbox list and returns the selected indices, or
// ErrCanceled when the user aborts. An empty (but confirmed) selection returns
// no indices and no error.
func RunMultiSelect(title string, items, descriptions []string) ([]int, error) {
	// The prompt is UI, not output: it goes to stderr so a command whose stdout
	// is piped or redirected (-o json > file) is not corrupted by the widget.
	p := tea.NewProgram(NewMultiSelect(title, items, descriptions), tea.WithOutput(os.Stderr))
	m, err := p.Run()
	if err != nil {
		return nil, err
	}
	model, ok := m.(MultiSelectModel)
	if !ok {
		return nil, fmt.Errorf("unexpected model type")
	}
	if model.Canceled() {
		return nil, ErrCanceled
	}
	return model.Selected(), nil
}

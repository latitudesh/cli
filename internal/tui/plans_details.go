package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// PlanDetailsModel shows full details for a plan
type PlanDetailsModel struct {
	plan     map[string]string
	title    string
	quitting bool
}

func NewPlanDetails(title string, plan map[string]string) PlanDetailsModel {
	return PlanDetailsModel{
		plan:  plan,
		title: title,
	}
}

func (m PlanDetailsModel) Init() tea.Cmd {
	return nil
}

func (m PlanDetailsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "enter", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m PlanDetailsModel) View() string {
	if m.quitting {
		return ""
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(PrimaryColor).
		MarginBottom(1).
		Padding(0, 1)

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(SecondaryColor).
		Width(20)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(PrimaryColor).
		Padding(1, 2).
		MarginTop(1)

	header := titleStyle.Render(m.title)

	fieldOrder := []struct {
		key   string
		label string
	}{
		{"SLUG", "Plan Slug"},
		{"ID", "Plan ID"},
		{"CPU", "CPU"},
		{"MEMORY", "Memory"},
		{"DRIVES", "Storage"},
		{"NIC", "Network"},
		{"FEATURES", "Features"},
		{"AVAILABLE IN", "Available In"},
		{"IN STOCK", "In Stock"},
		{"STOCK", "Stock Level"},
		{"PRICE/MO", "Price/Month"},
		{"REGION", "Region"},
		{"LOCATION", "Location"},
		{"RAM", "RAM"},
		{"PLAN", "Plan Name"},
	}

	var details []string
	for _, field := range fieldOrder {
		if value, ok := m.plan[field.key]; ok && value != "" {
			line := labelStyle.Render(field.label+":") + " " + valueStyle.Render(value)
			details = append(details, line)
		}
	}

	for key, value := range m.plan {
		found := false
		for _, field := range fieldOrder {
			if field.key == key {
				found = true
				break
			}
		}
		if !found && value != "" {
			line := labelStyle.Render(key+":") + " " + valueStyle.Render(value)
			details = append(details, line)
		}
	}

	content := strings.Join(details, "\n")
	box := boxStyle.Render(content)

	help := HelpStyle.Render("\npress any key to go back • q/esc: quit")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		box,
		help,
	)
}

// RunPlanDetails runs details for a plan
func RunPlanDetails(title string, plan map[string]string) {
	p := tea.NewProgram(
		NewPlanDetails(title, plan),
		tea.WithAltScreen(),
	)
	p.Run()
}

package tui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212")).
			MarginBottom(1)

	subtleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	okStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))

	missingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))

	keyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Bold(true)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1)

	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212")).
			Underline(true)

	inactiveTabStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

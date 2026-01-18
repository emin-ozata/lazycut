package ui

import "github.com/charmbracelet/lipgloss"

var (
	// Panel border style
	BorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1)

	// Panel title style
	TitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))
)

// FormatTitle formats a panel title
func FormatTitle(title string) string {
	return TitleStyle.Render(title)
}

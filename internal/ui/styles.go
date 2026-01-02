package ui

import "github.com/charmbracelet/lipgloss"

var (
	// Color palette
	primaryColor   = lipgloss.Color("63")  // Purple
	secondaryColor = lipgloss.Color("36")  // Cyan
	errorColor     = lipgloss.Color("196") // Red
	successColor   = lipgloss.Color("42")  // Green
	mutedColor     = lipgloss.Color("240") // Gray

	// Title style
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginBottom(1)

	// Subtitle style
	subtitleStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			MarginBottom(1)

	// Input label style
	labelStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true)

	// Error message style
	errorStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true).
			MarginTop(1)

	// Success message style
	successStyle = lipgloss.NewStyle().
			Foreground(successColor).
			Bold(true).
			MarginTop(1)

	// Help text style
	helpStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			MarginTop(1)

	// Selected item style
	selectedItemStyle = lipgloss.NewStyle().
				Foreground(primaryColor).
				Bold(true)

	// Normal item style
	itemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	// Dialog box style
	dialogBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(1, 2).
			MarginTop(1)

	// Button style
	buttonStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("0")).
			Background(primaryColor).
			Padding(0, 2).
			MarginRight(1)

	// Inactive button style
	inactiveButtonStyle = lipgloss.NewStyle().
				Foreground(mutedColor).
				Padding(0, 2).
				MarginRight(1)
)

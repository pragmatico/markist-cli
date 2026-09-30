package ui

import "github.com/charmbracelet/lipgloss"

// deviceCodeBoxStyle renders the device-flow user code large and boxed
// (plan Task 11), so it's unmistakable while the user copies it into the
// browser tab opposite their terminal.
var deviceCodeBoxStyle = lipgloss.NewStyle().
	Bold(true).
	Padding(1, 4).
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("205"))

// DeviceCodeBox renders code (already formatted, e.g. "WDJB-MJHT") inside a
// bordered box.
func DeviceCodeBox(code string) string {
	return deviceCodeBoxStyle.Render(code)
}

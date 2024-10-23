package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// lipgloss style
	redStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	greenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("28"))
	cyanStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("43"))
	yellowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
)

func printRed(msg string) {
	printLine(redStyle, msg)
}

func printCyan(msg string) {
	printLine(cyanStyle, msg)
}

func printYellow(msg string) {
	printLine(yellowStyle, msg)
}

func printLine(style lipgloss.Style, msg string) {
	fmt.Println(style.Render(msg))
}

// colorAroundTexts concats color around texts
func colorAroundTexts(s string, textColorMap map[string]*lipgloss.Style) string {
	for k, v := range textColorMap {
		if strings.Contains(s, k) {
			return strings.ReplaceAll(s, k, v.Render(k))
		}
	}
	return s
}

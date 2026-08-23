package ui

import "charm.land/lipgloss/v2"

// lipglossWidth measures a rendered line the way the terminal will.
func lipglossWidth(s string) int { return lipgloss.Width(s) }

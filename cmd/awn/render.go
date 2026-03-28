package main

import (
	"fmt"
	"strings"
)

// renderStatusBar produces a styled status line for the watch display.
func renderStatusBar(sessionID, state string) string {
	return fmt.Sprintf("\033[7m awn watch %s | %s \033[0m", sessionID, state)
}

// renderLines renders screen lines as ANSI output with clear-screen prefix.
func renderLines(lines []string) string {
	var b strings.Builder
	// Clear screen and move cursor to top-left.
	b.WriteString("\033[2J\033[H")
	for i, line := range lines {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprint(&b, line)
	}
	return b.String()
}

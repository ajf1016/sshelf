// Package ui provides terminal styling helpers built on charmbracelet/lipgloss.
package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Palette — all colour references in one place.
var (
	colGreen  = lipgloss.Color("2")
	colYellow = lipgloss.Color("3")
	colRed    = lipgloss.Color("1")
	colBlue   = lipgloss.Color("4")
	colGray   = lipgloss.Color("8")
	colBold   = lipgloss.Color("15")
)

// Base styles.
var (
	StyleHeader = lipgloss.NewStyle().Bold(true).Foreground(colBold)
	StyleMuted  = lipgloss.NewStyle().Foreground(colGray)
	StyleGreen  = lipgloss.NewStyle().Foreground(colGreen)
	StyleYellow = lipgloss.NewStyle().Foreground(colYellow)
	StyleRed    = lipgloss.NewStyle().Foreground(colRed)
	StyleBlue   = lipgloss.NewStyle().Foreground(colBlue)
	StyleBold   = lipgloss.NewStyle().Bold(true)
)

// Indicator returns a coloured status bullet.
func Indicator(active bool) string {
	if active {
		return StyleGreen.Render("*")
	}
	return " "
}

// CheckBadge returns a coloured PASS/WARN/FAIL badge.
func CheckBadge(status string) string {
	switch strings.ToUpper(status) {
	case "PASS":
		return StyleGreen.Render("PASS")
	case "WARN":
		return StyleYellow.Render("WARN")
	case "FAIL":
		return StyleRed.Render("FAIL")
	default:
		return StyleMuted.Render(status)
	}
}

// FormatAge returns a human-readable age string (e.g. "3d", "2h", "45m").
func FormatAge(d time.Duration) string {
	days := int(d.Hours() / 24)
	if days >= 1 {
		return fmt.Sprintf("%dd", days)
	}
	hours := int(d.Hours())
	if hours >= 1 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dm", int(d.Minutes()))
}

// Table renders a simple text table from headers and rows.
// Column widths are computed automatically to fit the widest cell.
func Table(headers []string, rows [][]string) string {
	cols := len(headers)
	widths := make([]int, cols)
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i := 0; i < cols && i < len(row); i++ {
			// Strip ANSI for width calculation.
			plain := stripANSI(row[i])
			if len(plain) > widths[i] {
				widths[i] = len(plain)
			}
		}
	}

	var sb strings.Builder

	// Header row.
	for i, h := range headers {
		cell := StyleHeader.Render(fmt.Sprintf("%-*s", widths[i], h))
		sb.WriteString(cell)
		if i < cols-1 {
			sb.WriteString("  ")
		}
	}
	sb.WriteString("\n")

	// Separator.
	for i, w := range widths {
		sb.WriteString(StyleMuted.Render(strings.Repeat("─", w)))
		if i < cols-1 {
			sb.WriteString("  ")
		}
	}
	sb.WriteString("\n")

	// Data rows.
	for _, row := range rows {
		for i := 0; i < cols; i++ {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			plainLen := len(stripANSI(cell))
			padding := widths[i] - plainLen
			if padding < 0 {
				padding = 0
			}
			sb.WriteString(cell)
			sb.WriteString(strings.Repeat(" ", padding))
			if i < cols-1 {
				sb.WriteString("  ")
			}
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// Section prints a named section heading.
func Section(name string) string {
	return StyleBlue.Bold(true).Render(name) + "\n"
}

// stripANSI removes ANSI escape sequences for width calculation.
func stripANSI(s string) string {
	var result strings.Builder
	inEsc := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if (s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= 'a' && s[i] <= 'z') {
				inEsc = false
			}
			continue
		}
		result.WriteByte(s[i])
	}
	return result.String()
}

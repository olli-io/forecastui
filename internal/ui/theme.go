// Package ui renders forecasts to the terminal, both interactively through
// Bubble Tea and as a one-shot dump.
package ui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/olli-io/forecastui/internal/render"
)

// gruvbox material (dark, medium) accents.
var palette = map[render.Colour]color.Color{
	render.Grey:   lipgloss.Color("#7c6f64"),
	render.FG:     lipgloss.Color("#d5c4a1"),
	render.Purple: lipgloss.Color("#d3869b"),
	render.Aqua:   lipgloss.Color("#89b482"),
	render.Green:  lipgloss.Color("#a9b665"),
	render.Yellow: lipgloss.Color("#d8a657"),
	render.Orange: lipgloss.Color("#e78a4e"),
	render.Red:    lipgloss.Color("#ea6962"),
	render.Blue:   lipgloss.Color("#7daea3"),
	render.Dim:    lipgloss.Color("#504945"),
}

var styles = func() map[render.Colour]lipgloss.Style {
	m := make(map[render.Colour]lipgloss.Style, len(palette))
	for k, c := range palette {
		m[k] = lipgloss.NewStyle().Foreground(c)
	}
	return m
}()

// Style returns the lipgloss style for a palette slot.
func Style(c render.Colour) lipgloss.Style { return styles[c] }

// Paint renders lines with colour, clipped to width. A width of zero means no
// limit; anything else guarantees no line can wrap, which would shear the
// braille grid across two terminal rows.
func Paint(lines []render.Line, colour bool, width int) string {
	var b strings.Builder
	for i, l := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(paintLine(l.Truncate(width), colour))
	}
	return b.String()
}

func paintLine(l render.Line, colour bool) string {
	var b strings.Builder
	for _, s := range l {
		if !colour || strings.TrimLeft(s.Text, " ⠀") == "" {
			b.WriteString(s.Text)
			continue
		}
		b.WriteString(styles[s.Colour].Render(s.Text))
	}
	return strings.TrimRight(b.String(), " ")
}

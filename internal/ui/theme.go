// Package ui renders forecasts to the terminal, both interactively through
// Bubble Tea and as a one-shot dump.
package ui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/olli-io/forecastui/internal/render"
	"github.com/olli-io/forecastui/internal/theme"
)

// The palette in force. It starts on the shipped default so anything rendered
// before Use is called still has colours to draw with.
var (
	active  = theme.Fallback()
	palette = active.Colours
	styles  = build(palette)
)

// Use installs a theme. Startup settles one before the program takes the
// screen; the picker calls it again from Update, which is the only other place
// it may be called from — these are package globals with no lock, and Bubble
// Tea's own goroutine is the one that reads them to draw.
func Use(t *theme.Theme) {
	active = t
	palette = t.Colours
	styles = build(palette)
}

// Active is the theme in force, so the picker can open on the row the chart is
// already wearing.
func Active() *theme.Theme { return active }

// ActiveName is the name of the theme in force.
func ActiveName() string { return active.Name }

func build(p map[render.Colour]color.Color) map[render.Colour]lipgloss.Style {
	m := make(map[render.Colour]lipgloss.Style, len(p))
	for k, c := range p {
		m[k] = lipgloss.NewStyle().Foreground(c)
	}
	return m
}

// Style returns the lipgloss style for a palette slot.
func Style(c render.Colour) lipgloss.Style { return styles[c] }

// Colour returns a palette slot as a plain colour, for the places one is not
// worn as a foreground style: a highlight band, a cursor.
func Colour(c render.Colour) color.Color { return palette[c] }

// Paint renders lines with colour, clipped to width so no line wraps and
// shears the braille grid. A width of zero means no limit.
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

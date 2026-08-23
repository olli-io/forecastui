package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/olli-io/forecastui/internal/render"
)

// The search prompt and the favourites list are drawn over the chart rather
// than in place of it, so the forecast is still on screen behind them.

// floatW is room for a place name and its coordinates side by side. Hits come
// back with the town and country around them, and a list of those is only
// worth reading when they are not all cut to the same stub.
const floatW = 84

// floatTop is the row a window hangs from — fixed, not centred, or the prompt
// would walk up the screen as the list grows. It clears the header box.
const floatTop = 5

// window is a floating panel. Its width is settled when the rows are built,
// since they are laid out to fill it.
type window struct {
	title string
	width int
	rows  []wrow
}

// wrow is one row of a window. Most are coloured spans; the search input
// renders its own escapes, so it comes through already painted.
type wrow struct {
	line render.Line
	raw  string
}

func textRow(l render.Line) wrow { return wrow{line: l} }
func rawRow(s string) wrow       { return wrow{raw: s} }

// floatWidth leaves room for the frame and a blank column either side.
func (a *App) floatWidth() int { return max(5, min(floatW, a.width-6)) }

// floatInner is the room for content: the width bar the walls and their
// inside padding.
func (a *App) floatInner() int { return max(1, a.floatWidth()-4) }

// floatRows is the content rows that fit; the frame takes one at each end.
func (a *App) floatRows() int { return max(1, a.height-floatTop-2) }

// render draws the window: a rounded frame with the title set into its top
// edge. The corners are round where every other box has square ones, so it
// reads as laid over the chart rather than as another panel in it.
func (w window) render(width int) string {
	inner := max(1, width-4)
	frame, name := Style(render.FG), Style(render.Yellow)

	// The title rides in the top edge, one dash in from the corner.
	head := "╭" + strings.Repeat("─", inner+2) + "╮"
	if title := " " + w.title + " "; lipgloss.Width(title)+1 <= inner+2 {
		head = frame.Render("╭─") + name.Render(title) +
			frame.Render(strings.Repeat("─", inner+1-lipgloss.Width(title))+"╮")
	} else {
		head = frame.Render(head)
	}

	out := []string{head}
	wall := frame.Render("│")
	for _, r := range w.rows {
		out = append(out, wall+" "+fitANSI(r.text(inner), inner)+" "+wall)
	}
	out = append(out, frame.Render("╰"+strings.Repeat("─", inner+2)+"╯"))

	// A blank column either side lifts the window clear of the braille.
	for i, l := range out {
		out[i] = " " + l + " "
	}
	return strings.Join(out, "\n")
}

// text paints one row, clipped to the window's inner width.
func (r wrow) text(inner int) string {
	if r.raw != "" {
		return r.raw
	}
	return Paint([]render.Line{r.line.Truncate(inner)}, true, 0)
}

// overlay composites a window over the chart, centred and hung from floatTop.
// The canvas is screen-sized, so nothing can push a line past the edge and
// shear the braille grid under it.
func (a *App) overlay(w window) string {
	base := a.chartView()
	win := w.render(w.width)
	x := max(0, (a.width-lipgloss.Width(win))/2)
	// A window taller than the room under floatTop is pushed up.
	y := max(0, min(floatTop, a.height-lipgloss.Height(win)))

	canvas := lipgloss.NewCanvas(a.width, a.height)
	canvas.Compose(lipgloss.NewCompositor(
		lipgloss.NewLayer(base),
		lipgloss.NewLayer(win).X(x).Y(y).Z(1),
	))
	return canvas.Render()
}

// highlight paints a line onto a background band, padded the whole way to the
// given width so it reads as a picked row rather than a smudge.
func highlight(l render.Line, w int, bg render.Colour) string {
	back := palette[bg]
	var b strings.Builder
	used := 0
	for _, s := range l.Truncate(w) {
		b.WriteString(Style(s.Colour).Background(back).Render(s.Text))
		used += lipgloss.Width(s.Text)
	}
	if used < w {
		b.WriteString(lipgloss.NewStyle().Background(back).
			Render(strings.Repeat(" ", w-used)))
	}
	return b.String()
}

// fitANSI pads or clips an already-painted string to an exact display width,
// so a window's right wall stands in one column.
func fitANSI(s string, w int) string {
	switch n := lipgloss.Width(s); {
	case n > w:
		return ansi.Truncate(s, w, "")
	case n < w:
		return s + strings.Repeat(" ", w-n)
	}
	return s
}

// fit does the same for plain text, clipping with an ellipsis so a cut place
// name says it was cut.
func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) > w {
		return ansi.Truncate(s, w-1, "") + "…"
	}
	return s + strings.Repeat(" ", w-lipgloss.Width(s))
}

// scrollTo returns the slice of n rows that fits in room, kept around the
// selection so the highlighted entry is always on screen.
func scrollTo(n, sel, room int) (lo, hi int) {
	if room >= n {
		return 0, n
	}
	lo = clamp(sel-room/2, 0, n-room)
	return lo, lo + room
}

package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/olli-io/forecastui/internal/render"
)

// Floating windows are the app's one departure from the stacked layout: the
// search prompt and the favourites list are drawn over the chart rather than
// in place of it, so the forecast they are about is still on screen behind
// them. The chart is the only view under them.

// floatW is how wide a floating window would like to be: room for a place
// name and its coordinates side by side without eliding the name. A hit comes
// back named with the town and country around it — "Kuopion lentoasema,
// Siilinjärvi, Suomi" — and a list of those is only worth reading when they
// are not all cut down to the same stub. A narrow terminal cuts it down anyway.
const floatW = 84

// floatTop is the row a window hangs from. It is fixed rather than centred
// because the search list grows and shrinks as you type, and a centred window
// would walk up the screen as it did — the prompt has to stay under the
// finger that is typing into it. Five rows down clears the header box and the
// shortcut line under it.
const floatTop = 5

// window is a floating panel: a title, and rows of content under it. Its
// width is settled when the rows are built, since they are laid out to fill it.
type window struct {
	title string
	width int
	rows  []wrow
}

// wrow is one row of a window. Most rows are coloured spans, like everywhere
// else in the app; the search input renders its own escapes, so it comes
// through already painted.
type wrow struct {
	line render.Line
	raw  string
}

func textRow(l render.Line) wrow { return wrow{line: l} }
func rawRow(s string) wrow       { return wrow{raw: s} }

// floatWidth is the outer width a window gets on this terminal, leaving room
// for the frame and a blank column either side of it.
func (a *App) floatWidth() int { return max(5, min(floatW, a.width-6)) }

// floatInner is the room a window has for content: its width bar the two
// walls and the space inside each of them.
func (a *App) floatInner() int { return max(1, a.floatWidth()-4) }

// floatRows is how many rows of content a window can show without running off
// the screen — the frame takes one row at each end.
func (a *App) floatRows() int { return max(1, a.height-floatTop-2) }

// render draws the window: a rounded frame with the title set into its top
// edge, the way the header box carries its range tabs. The corners are round
// where every other box in the app has square ones, so a floating window
// reads as something laid over the chart rather than another panel in it.
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

	// A blank column stands either side of the frame so the window lifts
	// clear of the braille instead of butting straight against it.
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

// overlay composites a window over the chart — centred across it, hung from
// floatTop — and clips the result to the terminal: the canvas is the size of
// the screen, so nothing a window carries can push a line past the edge and
// shear the braille grid under it.
func (a *App) overlay(w window) string {
	base := a.chartView()
	win := w.render(w.width)
	x := max(0, (a.width-lipgloss.Width(win))/2)
	// A window taller than the room under floatTop is pushed up rather than
	// off the bottom of the screen.
	y := max(0, min(floatTop, a.height-lipgloss.Height(win)))

	canvas := lipgloss.NewCanvas(a.width, a.height)
	canvas.Compose(lipgloss.NewCompositor(
		lipgloss.NewLayer(base),
		lipgloss.NewLayer(win).X(x).Y(y).Z(1),
	))
	return canvas.Render()
}

// highlight paints a line onto a background band, padded the whole way to the
// given width: a highlight that stopped at the last character would read as a
// smudge behind the text rather than as the row being picked out.
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
// so a window's right wall stands in one column however ragged its content.
func fitANSI(s string, w int) string {
	switch n := lipgloss.Width(s); {
	case n > w:
		return ansi.Truncate(s, w, "")
	case n < w:
		return s + strings.Repeat(" ", w-n)
	}
	return s
}

// fit does the same for plain text, clipping with an ellipsis rather than
// mid-word: it is place names that run long, and a cut one should say so.
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

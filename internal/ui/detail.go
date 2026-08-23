package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/olli-io/forecastui/internal/fmi"
	"github.com/olli-io/forecastui/internal/render"
)

// Field widths. Every reading is padded to a fixed size, so the box holds its
// shape as the cursor moves and the eye can stay on one column of the row.
const (
	feelsW = 8         // "(-15.1°)"
	popW   = 8         // " (100 %)"
	gustW  = 13        // "(12 m/s gust)"
	windW  = 9 + gustW // "↗ 12 m/s " and the gust after it
	cloudW = 12        // " 45 % cloudy"
)

// boxIndent is the column every box hanging off the chart stands in: the
// screen's own left edge rather than the y-axis out in the middle of the row,
// so the header and the detail read as the frame around the whole view and the
// chart hangs inside them. Their right walls still meet the chart's, which is
// what keeps the two tied together. keyIndent is where the shortcut list
// starts — two columns inside that, clear of the walls, the way the text
// inside a box is.
const (
	boxIndent = 0
	keyIndent = boxIndent + 2
)

// gap separates two readings.
var gap = render.Span{Text: "   ", Colour: render.Grey}

// detail describes the column under the cursor, and marks where it sits. Both
// views draw it: the app under a cursor the reader moves, the one-shot dump
// under the hour it is printed in.
func detail(cols []render.Column, o render.Opts, drawn, width int, nerd bool) []render.Line {
	if o.Cursor < 0 || o.Cursor >= len(cols) {
		return nil
	}
	c := cols[o.Cursor]

	// The footnote under the chart already says the week is drawn in 3 h
	// means; repeating it on every column only crowds the row.
	line := render.Line{
		{Text: c.At.Format("Mon 02 15:04"), Colour: render.FG},
		gap,
		// The glyph carries the weather on its own; spelling it out as well
		// only made the row jump about as the wording changed length. A moon
		// is purple here as it is in the strip above, or the same glyph would
		// be two colours at once on the one screen.
		{Text: string(fmi.Describe(c.Sym, c.Night).Rune(nerd)), Colour: glyphColour(c)},
		gap,
		{
			Text: num("%5.1f", 5, c.Temp) + " °C " +
				slot(feelsW, "("+num("%.1f", 0, c.Feels)+"°)"),
			Colour: render.TempColour(c.Temp.V, c.Temp.OK),
		},
		gap,
		// Zero reads as "0.0 mm/h" rather than "dry": the number is the same
		// width whether it rains or not.
		{Text: fmt.Sprintf("%4.1f mm/h", c.Rain), Colour: render.Blue},
		{Text: pad(popW, c.POP.OK, " (%.0f %%)", c.POP), Colour: render.Grey},
		gap,
		{Text: windText(c), Colour: render.WindColour(c.Wind.Or(0))},
		gap,
		{Text: pad(cloudW, c.Cloud.OK, "%3.0f %% cloudy", c.Cloud), Colour: render.Grey},
	}

	// The detail box carries the brightest frame on screen. It is the one
	// thing that changes as the cursor moves, so it reads first; the chart's
	// own walls and the cursor frame stay a shade back from it.
	return boxedTop(cursorMark(o), line, drawn, width, render.FG)
}

func (a *App) detail() []render.Line {
	return detail(a.cols, a.opts(), a.drawn(), a.width, a.nerd)
}

// cursorMark is the run of top edge that ends in a downward arrow directly
// under the cursor frame's upward one, so the frame and the box it feeds read
// as one gesture rather than two separate drawings.
func cursorMark(o render.Opts) render.Line {
	if o.Cursor < o.Start || o.Cursor >= o.Start+o.Count {
		return nil
	}
	// The edge starts one column past the frame's corner; the bars start at
	// AxisW. The difference is what the arrow's own column is measured from.
	lead := render.AxisW - boxIndent - 1
	return render.Line{
		{Text: strings.Repeat("─", lead+(o.Cursor-o.Start)*render.Step), Colour: render.FG},
		// The arrow belongs to the cursor frame it answers, not to the box it
		// stands in, so it is painted in that frame's shade.
		{Text: render.DownArrow, Colour: render.Dim},
	}
}

// glyphColour keeps the detail box's symbol in the foreground shade, bar the
// moon: the box does not tint by weather the way the strip does.
func glyphColour(c render.Column) render.Colour {
	if fmi.Moonlit(c.Sym, c.Night) {
		return render.Purple
	}
	return render.FG
}

// windText is the arrow, the speed, and the gust when it is worth naming. The
// compass letters are gone: the arrow says the same thing in one column.
func windText(c render.Column) string {
	if !c.Wind.OK {
		return strings.Repeat(" ", windW)
	}
	arrow := " "
	if c.Dir.OK {
		arrow = string(render.Arrow(c.Dir.V))
	}
	// A gust within half a metre of the sustained wind is rounding noise.
	gust := strings.Repeat(" ", gustW)
	if c.Gust.OK && c.Gust.V > c.Wind.V+0.5 {
		gust = slot(gustW, fmt.Sprintf("(%.0f m/s gust)", c.Gust.V))
	}
	return fmt.Sprintf("%s %2.0f m/s %s", arrow, c.Wind.V, gust)
}

// num formats a value to a fixed width, or a dash of that width when the
// forecast has none.
func num(format string, width int, v fmi.Val) string {
	if !v.OK {
		return fmt.Sprintf("%*s", width, "—")
	}
	return fmt.Sprintf(format, v.V)
}

// pad formats a value when it is known, and holds its space when it is not.
func pad(width int, ok bool, format string, v fmi.Val) string {
	if !ok {
		return strings.Repeat(" ", width)
	}
	return slot(width, fmt.Sprintf(format, v.V))
}

// slot left-aligns a reading in a fixed column, so a bracket opens tight
// against its number while the fields after it still line up.
func slot(width int, s string) string {
	return s + strings.Repeat(" ", max(0, width-lipgloss.Width(s)))
}

// boxedTop wraps a line in a frame that stands on the screen's left edge and
// closes on the chart's own right wall. cols is how many columns the chart
// actually draws, which is not always as many as the terminal would hold: a
// 24 h forecast in a wide window leaves the room to its right empty, and a box
// measured off the window rather than off the chart would reach past it. The
// frame is drawn in the given colour, and something can be set into its top
// edge from the left corner on — the header's range tabs ride there the way
// lazygit's do, in the border rather than on a row of their own. The line is
// folded onto more rows when the terminal is too narrow to hold it in one.
func boxedTop(top, l render.Line, cols, width int, frame render.Colour) []render.Line {
	indent := boxIndent
	// The chart's right wall stands at AxisW + columns*Step - 1, and the
	// frame's corner is indent + 1 + (inner + 2) columns along.
	inner := render.AxisW + cols*render.Step - indent - 4
	inner = min(inner, width-indent-4) // never wider than the terminal
	if inner < 1 {
		return []render.Line{l}
	}
	rows := fold(l, inner)

	pad := strings.Repeat(" ", indent)
	edge := strings.Repeat("─", inner+2)

	// The top edge, with whatever the caller wants set into it laid over the
	// dashes from the left corner on.
	head := render.Line{{Text: pad + "┌", Colour: frame}}
	if used := lipgloss.Width(top.Plain()); used > 0 && used <= inner+2 {
		head = append(head, top...)
		head = append(head, render.Span{Text: strings.Repeat("─", inner+2-used), Colour: frame})
	} else {
		head = append(head, render.Span{Text: edge, Colour: frame})
	}
	head = append(head, render.Span{Text: "┐", Colour: frame})

	out := []render.Line{head}
	for _, r := range rows {
		row := render.Line{{Text: pad + "│ ", Colour: frame}}
		row = append(row, r...)
		out = append(out, append(row, render.Span{
			Text:   strings.Repeat(" ", inner-lipgloss.Width(r.Plain())) + " │",
			Colour: frame,
		}))
	}
	return append(out, render.Line{{Text: pad + "└" + edge + "┘", Colour: frame}})
}

// fold breaks a line into rows no wider than w, at the gaps between fields.
func fold(l render.Line, w int) []render.Line {
	var (
		out  []render.Line
		cur  render.Line
		used int
	)
	for _, s := range l {
		if len(cur) == 0 && strings.TrimSpace(s.Text) == "" {
			continue // no field starts a row with the gap before it
		}
		if lipgloss.Width(s.Text) > w {
			// A field wider than the whole box: all we can do is clip it.
			if t := (render.Line{s}).Truncate(w); len(t) > 0 {
				s = t[0]
			}
		}
		sw := lipgloss.Width(s.Text)
		if used+sw > w && len(cur) > 0 {
			out, cur, used = append(out, cur), nil, 0
		}
		cur, used = append(cur, s), used+sw
	}
	if len(cur) > 0 {
		out = append(out, cur)
	}
	return out
}

package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/olli-io/forecastui/internal/fmi"
	"github.com/olli-io/forecastui/internal/render"
)

// Field widths. Every reading is padded to a fixed size, so the box holds its
// shape as the cursor moves.
const (
	feelsW = 8         // "(-15.1°)"
	popW   = 8         // " (100 %)"
	gustW  = 13        // "(12 m/s gust)"
	windW  = 9 + gustW // "↗ 12 m/s " and the gust after it
	cloudW = 12        // " 45 % cloudy"
)

// boxIndent is the column every box hanging off the chart stands in: the
// screen's left edge, so the header and detail frame the whole view. Their
// right walls still meet the chart's. keyIndent is where the shortcut list
// starts, two columns inside that.
const (
	boxIndent = 0
	keyIndent = boxIndent + 2
)

var gap = render.Span{Text: "   ", Colour: render.Grey}

// detail describes the column under the cursor, and marks where it sits.
func detail(cols []render.Column, o render.Opts, drawn, width int, nerd bool) []render.Line {
	if o.Cursor < 0 || o.Cursor >= len(cols) {
		return nil
	}
	c := cols[o.Cursor]

	line := render.Line{
		{Text: c.At.Format("Mon 02 15:04"), Colour: render.FG},
		gap,
		// No worded description: it made the row jump about as the wording
		// changed length.
		{Text: string(fmi.Describe(c.Sym, c.Night).Rune(nerd)), Colour: glyphColour(c)},
		gap,
		{
			Text: num("%5.1f", 5, c.Temp) + " °C " +
				slot(feelsW, "("+num("%.1f", 0, c.Feels)+"°)"),
			Colour: render.TempColour(c.Temp.V, c.Temp.OK),
		},
		gap,
		// Zero reads as "0.0 mm/h", so the field is one width either way.
		{Text: fmt.Sprintf("%4.1f mm/h", c.Rain), Colour: render.Blue},
		{Text: pad(popW, c.POP.OK, " (%.0f %%)", c.POP), Colour: render.Grey},
		gap,
		{Text: windText(c), Colour: render.WindColour(c.Wind.Or(0))},
		gap,
		{Text: pad(cloudW, c.Cloud.OK, "%3.0f %% cloudy", c.Cloud), Colour: render.Grey},
	}

	// The brightest frame on screen: it is the one thing that changes as the
	// cursor moves, so it reads first.
	return boxedTop(cursorMark(o), line, drawn, width, render.FG)
}

func (a *App) detail() []render.Line {
	return detail(a.cols, a.opts(), a.drawn(), a.width, a.nerd)
}

// cursorMark is the run of top edge ending in a downward arrow directly under
// the cursor frame's upward one.
func cursorMark(o render.Opts) render.Line {
	if o.Cursor < o.Start || o.Cursor >= o.Start+o.Count {
		return nil
	}
	// The edge starts one column past the frame's corner; the bars at AxisW.
	lead := render.AxisW - boxIndent - 1
	return render.Line{
		{Text: strings.Repeat("─", lead+(o.Cursor-o.Start)*render.Step), Colour: render.FG},
		// Lit like the cursor frame's arrow it answers, not like this box.
		{Text: render.DownArrow, Colour: render.Yellow},
	}
}

// glyphColour keeps the symbol in the foreground shade, bar the moon: the box
// does not tint by weather the way the strip does.
func glyphColour(c render.Column) render.Colour {
	if fmi.Moonlit(c.Sym, c.Night) {
		return render.Purple
	}
	return render.FG
}

// windText is the arrow, the speed, and the gust when it is worth naming.
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

// boxedTop wraps a line in a frame standing on the screen's left edge and
// closing on the chart's right wall. cols is how many columns the chart draws,
// not how many the terminal would hold, or the box would reach past it. top is
// set into the frame's top edge; the line is folded when the terminal is
// too narrow.
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
			// Wider than the whole box: all we can do is clip it.
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

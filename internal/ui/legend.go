package ui

import (
	"fmt"

	"github.com/olli-io/forecastui/internal/render"
)

// Header is the place and time-span line above a one-shot chart, indented to
// stand clear of the left edge. The interactive view boxes headerText instead.
func Header(place string, cols []render.Column, slots bool) render.Line {
	return append(render.Line{{Text: "  ", Colour: render.Grey}},
		headerText(place, cols, slots)...)
}

// headerText names the place and the span the chart covers.
func headerText(place string, cols []render.Column, slots bool) render.Line {
	if len(cols) == 0 {
		return render.Line{{Text: place, Colour: render.FG}}
	}
	layout := "15:04"
	if slots {
		layout = "2 Jan"
	}
	first, last := cols[0].At, cols[len(cols)-1].At
	span := fmt.Sprintf("%s %s → %s %s, local time",
		first.Format("Mon"), first.Format(layout),
		last.Format("Mon"), last.Format(layout))
	return render.Line{
		{Text: place + "  ", Colour: render.FG},
		{Text: span, Colour: render.Grey},
	}
}

// Legend is the colour-ramp and totals line below a chart.
func Legend(cols []render.Column, sc render.Scale, slots bool) render.Line {
	line := render.Line{{Text: "  ", Colour: render.Grey}}
	for _, c := range []render.Colour{
		render.Purple, render.Aqua, render.Green,
		render.Yellow, render.Orange, render.Red,
	} {
		line = append(line, render.Span{Text: "█", Colour: c})
	}
	// The axis prints the bounds a step to the left, and they now cover the
	// feels-like bars too, so naming a "temperature range" here would be both
	// a repetition and a slight lie.
	line = append(line,
		render.Span{Text: " temperature   ", Colour: render.FG},
		render.Span{Text: "█", Colour: render.Dim},
		render.Span{Text: " feels like   ", Colour: render.FG},
		render.Span{Text: "█", Colour: render.Blue},
		render.Span{Text: " rain ", Colour: render.FG})

	if sc.RainMax <= 0 {
		return append(line, render.Span{Text: "none forecast", Colour: render.Grey})
	}
	// In slot mode each column stands for three hours of accumulation.
	var total float64
	for _, c := range cols {
		total += c.Rain
	}
	if slots {
		total *= 3
	}
	return append(line, render.Span{
		Text:   fmt.Sprintf("peak %.1f mm/h, %.1f mm total", sc.RainMax, total),
		Colour: render.FG,
	})
}

// Note is the explanatory footnote shown under an aggregated chart.
func Note(slots bool) render.Line {
	if !slots {
		return nil
	}
	return render.Line{{
		Text:   "  each column is a 3 h rolling mean, on the clock: 00, 03, 06 …",
		Colour: render.Grey,
	}}
}

// blank is a spacer row.
func blank() render.Line { return render.Line{{Text: "", Colour: render.Grey}} }

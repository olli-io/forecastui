package ui

import (
	"fmt"

	"github.com/olli-io/forecastui/internal/render"
)

// Header is the place and time-span line above a chart as a plain indented
// row, for the daily table. The chart views box headerText instead.
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

// blank is a spacer row.
func blank() render.Line { return render.Line{{Text: "", Colour: render.Grey}} }

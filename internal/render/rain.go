package render

import (
	"fmt"
	"math"

	"github.com/olli-io/forecastui/internal/fmi"
)

// Rain draws the precipitation panel: the rate in the left cell, and beside it
// in grey the probability, on an unlabelled 0–100 axis of its own. It is drawn
// even for a wholly dry forecast, so the stack never shifts under the reader.
func Rain(cols []Column, sc Scale, o Opts) []Line {
	if o.CellH <= 0 {
		o.CellH = 4
	}
	lo, hi := window(len(cols), o.Start, o.Count)
	if lo == hi {
		return nil
	}
	dots := o.CellH * 4

	bar := func(mm float64) int {
		if mm <= 0 || sc.RainMax <= 0 {
			return 0
		}
		return max(1, int(math.Round(mm/sc.RainMax*float64(dots))))
	}
	chance := func(v fmi.Val) int {
		if !v.OK || v.V <= 0 {
			return 0
		}
		return min(dots, max(1, int(math.Round(v.V/100*float64(dots)))))
	}

	// The divider rises one row from the rule — shorter than the chart's.
	ends := func(i int) bool { return i+1 < len(cols) && cols[i+1].NewDay }

	var out []Line
	for r := 0; r < o.CellH; r++ {
		label, labelCol := Gutter(""), Grey
		switch r {
		case 0:
			label = rateLabel(sc.RainMax)
		case 2:
			label, labelCol = Gutter("rain "), Dim
		}
		line := Line{{label, labelCol}, {vert + " ", Grey}}

		for i := lo; i < hi; i++ {
			c := cols[i]
			line = append(line,
				brailleSpan(cell(bar(c.Rain), r, o.CellH), Blue),
				brailleSpan(cell(chance(c.POP), r, o.CellH), Dim),
				Span{divider(ends(i), i == hi-1, r, o.CellH), Grey})
		}
		line = append(line, Span{vert, Grey})
		out = append(out, line)
	}

	return out
}

// RainLabels is the rate under each column, in whole mm/h. A dry hour prints a
// grey 0, so the row keeps a number under every column.
func RainLabels(cols []Column, sc Scale, o Opts) Line {
	lo, hi := window(len(cols), o.Start, o.Count)
	if lo == hi {
		return nil
	}
	line := Line{rowLabel("rain")}
	for i := lo; i < hi; i++ {
		// Blue means it rains whatever the rounding says: drizzle prints a blue
		// 0, and only a dry hour gets the grey one.
		mm, col := cols[i].Rain, Blue
		if mm <= 0 {
			col = Grey
		}
		line = append(line, Span{fmt.Sprintf("%-*.0f", Step, mm), col})
	}
	return line
}

// rateLabel is the peak rain rate, fitted to the gutter. The unit is shortened
// to "mm"; the legend spells out that it is per hour.
func rateLabel(mm float64) string {
	s := fmt.Sprintf("%.1f mm", mm)
	if mm >= 10 {
		s = fmt.Sprintf("%.0f mm", mm)
	}
	return Gutter(s)
}

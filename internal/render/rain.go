package render

import (
	"fmt"
	"math"
	"strings"

	"github.com/olli-io/forecastui/internal/fmi"
)

// Rain draws the precipitation panel: the forecast rate in the left cell, the
// way the temperature and the sustained wind stand in theirs, and beside it in
// grey the probability of precipitation. The chance runs on a fixed 0–100 axis
// of its own, unlabelled: it is there to be read against the other columns'
// chances, and putting a second scale in the gutter would only invite it to be
// read against the millimetres. The panel hangs between the temperature chart
// and the wind panel, on the same time line, and a dry forecast draws nothing
// at all — the legend says so instead of a strip of empty box saying it for
// eighteen columns running.
func Rain(cols []Column, sc Scale, o Opts) []Line {
	if o.CellH <= 0 {
		o.CellH = 4
	}
	lo, hi := window(len(cols), o.Start, o.Count)
	if lo == hi || sc.RainMax <= 0 {
		return nil
	}
	dots := o.CellH * 4

	bar := func(mm float64) int {
		if mm <= 0 {
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

	// A day ends where the next one begins, and its divider rises one row from
	// the rule — a shorter stroke than the chart's, for a shorter panel.
	ends := func(i int) bool { return i+1 < len(cols) && cols[i+1].NewDay }

	var out []Line
	for r := 0; r < o.CellH; r++ {
		label := Gutter("")
		if r == 0 {
			label = rateLabel(sc.RainMax)
		}
		line := Line{{label + vert + " ", Grey}}

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

// RainLabels is the rate under each column, in whole mm/h. It reads under the
// temperature row. A dry hour still prints its 0, in grey rather than blue, so
// the row keeps a number under every column and the eye can run along it —
// which is also how a wet hour that rounds down to zero stays distinguishable
// from a dry one. Only a forecast that is dry from end to end drops the row
// altogether, which keeps it from appearing and vanishing as the window
// scrolls.
func RainLabels(cols []Column, sc Scale, o Opts) Line {
	lo, hi := window(len(cols), o.Start, o.Count)
	if lo == hi || sc.RainMax <= 0 {
		return nil
	}
	line := Line{{strings.Repeat(" ", AxisW), Grey}}
	for i := lo; i < hi; i++ {
		// Blue means it rains, whatever the rounding says: an hour of drizzle
		// prints a blue 0, and only a dry hour gets the grey one.
		mm, col := cols[i].Rain, Blue
		if mm <= 0 {
			col = Grey
		}
		line = append(line, Span{fmt.Sprintf("%-*.0f", Step, mm), col})
	}
	return line
}

// rateLabel is the peak rain rate in the columns the gutter allows. The unit
// is dropped to "mm" — the legend spells out that it is per hour, and a heavy
// enough downpour needs those two columns for its digits.
func rateLabel(mm float64) string {
	s := fmt.Sprintf("%.1f mm", mm)
	if mm >= 10 {
		s = fmt.Sprintf("%.0f mm", mm)
	}
	return Gutter(s)
}

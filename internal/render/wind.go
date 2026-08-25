package render

import (
	"fmt"
	"math"
	"strings"

	"github.com/olli-io/forecastui/internal/fmi"
)

// arrows point where the wind is blowing, indexed by compass octant.
var arrows = [8]rune{'↓', '↙', '←', '↖', '↑', '↗', '→', '↘'}

var compass = [8]string{"N", "NE", "E", "SE", "S", "SW", "W", "NW"}

// octant bins a bearing into eight points.
func octant(deg float64) int {
	o := int(math.Round(deg/45)) % 8
	if o < 0 {
		o += 8
	}
	return o
}

// Arrow is the glyph for wind coming *from* the given bearing; like a weather
// map, it points where the wind is going.
func Arrow(fromDeg float64) rune { return arrows[octant(fromDeg)] }

// Compass names the direction wind comes from, e.g. "NW".
func Compass(fromDeg float64) string { return compass[octant(fromDeg)] }

// WindColour grades wind speed roughly by Beaufort feel.
func WindColour(ms float64) Colour {
	switch {
	case ms < 4:
		return Aqua
	case ms < 8:
		return Green
	case ms < 12:
		return Yellow
	case ms < 17:
		return Orange
	}
	return Red
}

// Wind draws the wind panel: sustained speed on the left, hourly maximum gust
// on the right, sharing one scale so gusts read as the margin over the wind.
func Wind(cols []Column, sc Scale, o Opts) []Line {
	if o.CellH <= 0 {
		o.CellH = 4
	}
	lo, hi := window(len(cols), o.Start, o.Count)
	if lo == hi || sc.WindMax <= 0 {
		return nil
	}
	dots := o.CellH * 4

	bar := func(v fmi.Val) int {
		if !v.OK || v.V <= 0 {
			return 0
		}
		return max(1, int(math.Round(v.V/sc.WindMax*float64(dots))))
	}

	// One-row divider, as in the rain panel: too short for the chart's stroke.
	ends := func(i int) bool { return i+1 < len(cols) && cols[i+1].NewDay }

	var out []Line
	for r := 0; r < o.CellH; r++ {
		label, labelCol := Gutter(""), Grey
		switch r {
		case 0:
			label = Gutter(fmt.Sprintf("%.0f m/s", sc.WindMax))
		case 2:
			label, labelCol = Gutter("wind "), Dim
		}
		line := Line{{label, labelCol}, {vert + " ", Grey}}

		for i := lo; i < hi; i++ {
			c := cols[i]
			speed := cell(bar(c.Wind), r, o.CellH)
			gust := cell(bar(c.Gust), r, o.CellH)
			line = append(line,
				brailleSpan(speed, WindColour(c.Wind.Or(0))),
				brailleSpan(gust, Dim),
				Span{divider(ends(i), i == hi-1, r, o.CellH), Grey})
		}
		line = append(line, Span{vert, Grey})
		out = append(out, line)
	}

	return out
}

// WindDirs is the row of direction arrows, below the panel's rule.
func WindDirs(cols []Column, o Opts) Line {
	lo, hi := window(len(cols), o.Start, o.Count)
	if lo == hi {
		return nil
	}
	return labelRow(hi-lo, "wdir", Grey, func(i int) (string, string) {
		c := cols[lo+i]
		if !c.Dir.OK {
			return "", ""
		}
		return string(Arrow(c.Dir.V)), ""
	})
}

// WindSpeeds is the sustained speed under each column, rounded and tinted like
// the bar. Gusts are left to the bars: two numbers would not fit in four cells.
func WindSpeeds(cols []Column, o Opts) Line {
	lo, hi := window(len(cols), o.Start, o.Count)
	if lo == hi {
		return nil
	}
	line := Line{rowLabel("wind")}
	for i := lo; i < hi; i++ {
		c := cols[i]
		if !c.Wind.OK {
			line = append(line, Span{strings.Repeat(" ", Step), Grey})
			continue
		}
		line = append(line, Span{
			fmt.Sprintf("%-*.0f", Step, c.Wind.V),
			WindColour(c.Wind.V),
		})
	}
	return line
}

// divider fills the gap after a column: blank, or the day divider in its last
// character on the panel's bottom row.
func divider(ends, last bool, row, cellH int) string {
	g := gapAfter(last)
	if !ends || row != cellH-1 {
		return strings.Repeat(" ", g)
	}
	return strings.Repeat(" ", g-1) + vert
}

// brailleSpan renders one cell, or a blank when nothing is lit.
func brailleSpan(bits rune, col Colour) Span {
	if bits == 0 {
		return Span{" ", Grey}
	}
	return Span{string(brailleBase + bits), col}
}

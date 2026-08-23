package render

import (
	"strings"

	"github.com/olli-io/forecastui/internal/fmi"
)

// Sky draws one row: the forecast symbol per column. Both symbol forms are
// single-width, so the row stays on the chart's grid either way.
func Sky(cols []Column, o Opts) []Line {
	lo, hi := window(len(cols), o.Start, o.Count)
	if lo == hi {
		return nil
	}

	// The row hangs outside the chart's box, so the gutter names it rather
	// than closing it with a wall.
	sym := Line{rowLabel("symb")}
	for i := lo; i < hi; i++ {
		s := fmi.Describe(cols[i].Sym, cols[i].Night)
		sym = append(sym, Span{string(s.Rune(o.Nerd)) + " ",
			symbolColour(cols[i].Sym, cols[i].Night)},
			Span{strings.Repeat(" ", Gap), Grey})
	}
	return []Line{sym}
}

// symbolColour tints the symbol by what it means: dry, wet, frozen or violent.
// A moon is tinted as a moon; nothing else in the row marks a dark hour.
func symbolColour(code int, night bool) Colour {
	if fmi.Moonlit(code, night) {
		return Purple
	}
	switch {
	case code == 1:
		return Yellow
	case code == 2:
		return FG
	case code == 3, code == 91, code == 92:
		return Grey
	case code >= 61 && code <= 64:
		return Red
	case code >= 41 && code <= 53, code >= 71 && code <= 83:
		return Aqua
	case code >= 21 && code <= 33:
		return Blue
	}
	return Grey
}

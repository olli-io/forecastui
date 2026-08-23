package render

import "strings"

// Rune forms of the frame glyphs, for overlaying one stroke on another.
var (
	gVert    = firstRune(vert)
	gHoriz   = firstRune(horiz)
	gCorner  = firstRune(corner)
	gRCorner = firstRune(rCorner)
	gDayEnd  = firstRune(dayEnd)
	gTick    = firstRune(tick)
)

const (
	gCross = '┼'
	gTeeL  = '├'
	gTeeR  = '┤'
	gTopL  = '┌'
	gTopR  = '┐'
	gArrow = '▲'
)

// DownArrow points back up at the cursor column from a box below the chart.
const DownArrow = "▼"

func firstRune(s string) rune { return []rune(s)[0] }

// Cursor frames the selected column across every row it is given, capped above
// and closed below by an arrow. The walls stand in the gaps either side, so
// they never cover a bar.
func Cursor(lines []Line, cols []Column, o Opts) []Line {
	lo, hi := window(len(cols), o.Start, o.Count)
	if lo == hi || o.Cursor < lo || o.Cursor >= hi {
		return lines
	}
	left := AxisW + (o.Cursor-lo)*Step - 1
	right := left + Step - 1

	// The cap above the frame, and the foot below it with the arrow under the
	// left cell, where the temperature bar stands.
	edge := func(l, mid, r rune) Line {
		rs, cs := explode(nil, right+1)
		for x := left; x <= right; x++ {
			rs[x], cs[x] = gHoriz, Dim
		}
		rs[left], rs[left+1], rs[right] = l, mid, r
		// The arrow names the picked column, so it carries the accent.
		if mid == gArrow {
			cs[left+1] = Yellow
		}
		return implode(rs, cs)
	}

	out := make([]Line, 0, len(lines)+2)
	out = append(out, edge(gTopL, gHoriz, gTopR))
	for _, l := range lines {
		rs, cs := explode(l, right+1)
		for _, x := range [2]int{left, right} {
			rs[x], cs[x] = wallOver(rs[x]), Dim
		}
		out = append(out, implode(rs, cs))
	}

	return append(out, edge(gCorner, gArrow, gRCorner))
}

// wallOver merges a vertical stroke with the glyph it lands on.
func wallOver(old rune) rune {
	switch old {
	case gHoriz, gDayEnd, gTick:
		return gCross
	case gCorner:
		return gTeeL
	case gRCorner:
		return gTeeR
	}
	return gVert
}

// explode spreads a line into runes and their colours, padded to n cells.
// Every glyph the chart draws is one cell wide, so runes index columns.
func explode(l Line, n int) ([]rune, []Colour) {
	var (
		rs []rune
		cs []Colour
	)
	for _, s := range l {
		for _, r := range s.Text {
			rs = append(rs, r)
			cs = append(cs, s.Colour)
		}
	}
	for len(rs) < n {
		rs = append(rs, ' ')
		cs = append(cs, Grey)
	}
	return rs, cs
}

// implode is explode's inverse, merging neighbours that share a colour.
func implode(rs []rune, cs []Colour) Line {
	var (
		out Line
		b   strings.Builder
	)
	for i, r := range rs {
		if i > 0 && cs[i] != cs[i-1] {
			out = append(out, Span{b.String(), cs[i-1]})
			b.Reset()
		}
		b.WriteRune(r)
	}
	if b.Len() > 0 {
		out = append(out, Span{b.String(), cs[len(cs)-1]})
	}
	return out
}

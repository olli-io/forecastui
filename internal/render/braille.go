package render

import (
	"fmt"
	"math"
	"strings"

	"charm.land/lipgloss/v2"
)

// Colour names a palette slot. The render package never emits escape codes;
// internal/ui maps these onto lipgloss.
type Colour uint8

const (
	Grey Colour = iota
	FG
	Purple
	Aqua
	Green
	Yellow
	Orange
	Red
	Blue // rain
	Dim  // faint accent, e.g. probability of precipitation
)

// Span is a run of text sharing one colour.
type Span struct {
	Text   string
	Colour Colour
}

// Line is one terminal row.
type Line []Span

// Plain renders a line without colour.
func (l Line) Plain() string {
	var b strings.Builder
	for _, s := range l {
		b.WriteString(s.Text)
	}
	return b.String()
}

// Plain renders lines without colour, newline separated.
func Plain(lines []Line) string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = strings.TrimRight(l.Plain(), " ")
	}
	return strings.Join(out, "\n")
}

// Braille dot bits, top to bottom, for the left and right columns of a cell.
var (
	leftDots  = [4]rune{0x01, 0x02, 0x04, 0x40}
	rightDots = [4]rune{0x08, 0x10, 0x20, 0x80}
)

const (
	brailleBase = 0x2800
	// Step is one column: two braille cells plus the gap after them. Gap's
	// last character carries the day divider.
	Step = 4
	Gap  = Step - 2
	// AxisW is the left gutter: a seven-wide label ("-10.4°C"), the axis, and
	// one blank column for the cursor frame.
	AxisW   = 9
	AxisCol = AxisW - 2 // where the y-axis itself stands
)

// gapAfter is the blank room after a column's two cells. The last column gets
// one less, so the right wall stands as close to the bars as the y-axis does.
func gapAfter(last bool) int {
	if last {
		return Gap - 1
	}
	return Gap
}

// cell returns the braille bits for a bar `dots` tall at cell row `row`,
// counting from the top.
func cell(dots, row, cellH int) rune {
	base := 4 * (cellH - 1 - row)
	var bits rune
	for s := 0; s < 4; s++ {
		if base+3-s < dots {
			bits |= leftDots[s] | rightDots[s]
		}
	}
	return bits
}

// TempColour is the cold-to-hot ramp. The cold end runs purple → aqua, not
// through blue, which is reserved for rain.
func TempColour(v float64, ok bool) Colour {
	if !ok {
		return Grey
	}
	switch {
	case v < -10:
		return Purple
	case v < 0:
		return Aqua
	case v < 8:
		return Green
	case v < 16:
		return Yellow
	case v < 24:
		return Orange
	}
	return Red
}

// Opts controls one chart rendering. Start and Count select the visible window
// into cols; the scale is shared, so panning does not rescale the bars.
type Opts struct {
	Start  int
	Count  int
	CellH  int  // braille cells tall
	Slots  bool // 3 h aggregated mode
	Nerd   bool // the terminal can draw Nerd Font glyphs
	Cursor int  // index into cols of the highlighted column; -1 for none
}

const (
	vert    = "│"
	corner  = "└"
	horiz   = "─"
	tick    = "┼"
	dayEnd  = "┴"
	rCorner = "┘"
)

// Chart draws the temperature bars for cols[Start:Start+Count]: the air
// temperature, and beside it the apparent temperature in faint grey. The rule
// and hour labels below them come from Axis.
func Chart(cols []Column, sc Scale, o Opts) []Line {
	if o.CellH <= 0 {
		o.CellH = 7
	}
	lo, hi := window(len(cols), o.Start, o.Count)
	if lo == hi {
		return nil
	}
	dots := o.CellH * 4

	// Where 0 °C falls on the dot grid. Only a reference line: it goes in the
	// gaps and through cells no bar reaches, never over a bar.
	zeroRow, zeroSub := -1, 0
	if z := sc.pos(0, dots); z >= 0 && z <= float64(dots-1) {
		idx := int(math.Round(z))
		zeroRow, zeroSub = o.CellH-1-idx/4, 3-idx%4
	}
	zeroBits := func(row int) rune {
		if row != zeroRow {
			return 0
		}
		return leftDots[zeroSub] | rightDots[zeroSub]
	}

	tempDots := func(c Column) int {
		if !c.Temp.OK {
			return 0
		}
		return max(1, int(math.Round(sc.pos(c.Temp.V, dots))))
	}
	feelsDots := func(c Column) int {
		if !c.Feels.OK {
			return 0
		}
		return max(1, int(math.Round(sc.pos(c.Feels.V, dots))))
	}

	// A day ends where the next begins; that column's gap becomes the divider.
	// Testing the full slice keeps it visible at the right edge of the viewport.
	ends := func(i int) bool { return i+1 < len(cols) && cols[i+1].NewDay }

	var out []Line
	for r := 0; r < o.CellH; r++ {
		// Only the top of the scale is labelled here; the bottom belongs on the
		// rule under the panel, where the bars actually stand. The panel's name
		// hangs two blank rows below that, unless the zero line has claimed the
		// row; a chart squeezed shorter than four cells goes unnamed.
		label, labelCol := Gutter(""), Grey
		switch {
		case r == 0:
			label = Gutter(fmt.Sprintf("%.1f°C", sc.Hi))
		case r == zeroRow:
			label = Gutter("0°C")
		case r == 3:
			label, labelCol = Gutter("temp "), Dim
		}
		axis := vert
		if r == zeroRow {
			axis = tick
		}
		// The blank column after the axis, carrying the zero line across it.
		z := zeroBits(r)
		pad := " "
		if z != 0 {
			pad = string(brailleBase + z)
		}
		line := Line{{label, labelCol}, {axis + pad, Grey}}
		for i := lo; i < hi; i++ {
			c := cols[i]
			for _, bar := range [2]struct {
				bits rune
				col  Colour
			}{
				{cell(tempDots(c), r, o.CellH), TempColour(c.Temp.V, c.Temp.OK)},
				{cell(feelsDots(c), r, o.CellH), Dim},
			} {
				if bar.bits != 0 {
					line = append(line, Span{string(brailleBase + bar.bits), bar.col})
				} else {
					line = append(line, Span{string(rune(brailleBase + z)), Grey})
				}
			}
			// The day divider is a two-row tick, not a full wall.
			g := gapAfter(i == hi-1)
			switch {
			case ends(i) && r >= o.CellH-2:
				line = append(line, Span{strings.Repeat(" ", g-1) + vert, Grey})
			case z != 0:
				line = append(line, Span{strings.Repeat(string(rune(brailleBase+z)), g), Grey})
			default:
				line = append(line, Span{strings.Repeat(" ", g), Grey})
			}
		}

		line = append(line, Span{vert, Grey})
		out = append(out, line)
	}

	return out
}

// Gutter right-aligns an axis label before the axis, so every panel's numbers
// stand in the same column.
func Gutter(label string) string { return fmt.Sprintf("%*s", AxisCol, label) }

// rowLabel names one of the rows hanging under the boxes, in the panels'
// gutter. An empty name leaves the gutter blank.
func rowLabel(name string) Span { return Span{Gutter(name) + "  ", Dim} }

// Rule closes a panel's box, meeting each day divider with a tick. Its label
// is the bottom of that panel's y-axis, since the bars stand on the rule.
func Rule(cols []Column, o Opts, label string) Line {
	lo, hi := window(len(cols), o.Start, o.Count)
	if lo == hi {
		return nil
	}
	ends := func(i int) bool { return i+1 < len(cols) && cols[i+1].NewDay }

	var rule strings.Builder
	rule.WriteString(Gutter(label))
	rule.WriteString(corner)
	rule.WriteString(horiz)
	for i := lo; i < hi; i++ {
		rule.WriteString(strings.Repeat(horiz, 1+gapAfter(i == hi-1)))
		if ends(i) {
			rule.WriteString(dayEnd)
		} else {
			rule.WriteString(horiz)
		}
	}
	rule.WriteString(rCorner)
	return Line{{rule.String(), Grey}}
}

// HourLabels is the day row and the time row under it, drawn beneath the
// bottom panel of a stack.
func HourLabels(cols []Column, o Opts) []Line {
	lo, hi := window(len(cols), o.Start, o.Count)
	if lo == hi {
		return nil
	}
	n := hi - lo
	// Day names are wide enough to run under the cursor frame.
	var avoid []int
	if o.Cursor >= lo && o.Cursor < hi {
		x := (o.Cursor - lo) * Step
		avoid = append(avoid, x-1, x+Step-2)
	}
	days := labelRow(n, "", FG, func(i int) (string, string) {
		// The leftmost column always names its day, or the row stays blank
		// until the next midnight scrolls into view.
		if i > 0 && !cols[lo+i].NewDay {
			return "", ""
		}
		return cols[lo+i].At.Format("Mon 02"), cols[lo+i].At.Format("02")
	}, avoid...)
	hours := labelRow(n, "time", Grey, func(i int) (string, string) {
		return cols[lo+i].At.Format("15"), ""
	})
	return []Line{days, hours}
}

// TempLabels is the temperature under each column, rounded and tinted like
// the bar it belongs to.
func TempLabels(cols []Column, o Opts) Line {
	lo, hi := window(len(cols), o.Start, o.Count)
	if lo == hi {
		return nil
	}
	line := Line{rowLabel("temp")}
	for i := lo; i < hi; i++ {
		c := cols[i]
		if !c.Temp.OK {
			line = append(line, Span{strings.Repeat(" ", Step), Grey})
			continue
		}
		line = append(line, Span{
			fmt.Sprintf("%-*.0f", Step, c.Temp.V),
			TempColour(c.Temp.V, true),
		})
	}
	return line
}

// Axis closes a lone chart: rule, hour labels, temperatures.
func Axis(cols []Column, sc Scale, o Opts) []Line {
	rule := Rule(cols, o, fmt.Sprintf("%.1f°", sc.Lo))
	if rule == nil {
		return nil
	}
	return append(append([]Line{rule}, HourLabels(cols, o)...), TempLabels(cols, o))
}

// labelRow lays text under each column. Labels may spill into the following
// columns' space, so when two fall close together the earlier one falls back
// to its short form.
func labelRow(n int, name string, col Colour, at func(i int) (full, short string), avoid ...int) Line {
	// Columns the cursor frame owns; a label cut by one falls back to short.
	blocked := func(start, width int) bool {
		for _, x := range avoid {
			if x >= start && x < start+width {
				return true
			}
		}
		return false
	}

	row := []rune(strings.Repeat(" ", n*Step))

	// Where the next label begins, so each knows how much room it has.
	next := make([]int, n)
	limit := len(row)
	for i := n - 1; i >= 0; i-- {
		next[i] = limit
		if full, _ := at(i); full != "" {
			limit = i * Step
		}
	}

	for i := 0; i < n; i++ {
		full, short := at(i)
		if full == "" {
			continue
		}
		start := i * Step
		text := []rune(full)
		// Leave a space before the next label rather than butting against it.
		if short != "" && (start+len(text)+1 > next[i] || blocked(start, len(text))) {
			text = []rune(short)
		}
		for j, r := range text {
			if start+j >= len(row) {
				break
			}
			row[start+j] = r
		}
	}
	return Line{rowLabel(name),
		{strings.TrimRight(string(row), " "), col}}
}

// window clamps a viewport request to the available columns.
func window(total, start, count int) (int, int) {
	if total == 0 {
		return 0, 0
	}
	if count <= 0 || count > total {
		count = total
	}
	if start < 0 {
		start = 0
	}
	if start+count > total {
		start = total - count
	}
	return start, start + count
}

// MaxCols is the widest a chart is ever drawn: two days of hourly readings.
// Longer ranges scroll rather than widening the box.
const MaxCols = 48

// Fits reports how many columns a chart can show in the given terminal width,
// capped at MaxCols.
func Fits(width int) int {
	n := (width - AxisW) / Step
	if n < 1 {
		return 1
	}
	return min(n, MaxCols)
}

// Truncate clips a line to a display width, dropping whole runes so a braille
// cell is never cut in half. A width of zero or less leaves the line alone.
func (l Line) Truncate(width int) Line {
	if width <= 0 {
		return l
	}
	out := make(Line, 0, len(l))
	used := 0
	for _, s := range l {
		if used >= width {
			break
		}
		w := lipgloss.Width(s.Text)
		if used+w <= width {
			out = append(out, s)
			used += w
			continue
		}
		// This span straddles the edge: keep as many runes as still fit.
		var b strings.Builder
		for _, r := range s.Text {
			rw := lipgloss.Width(string(r))
			if used+rw > width {
				break
			}
			b.WriteRune(r)
			used += rw
		}
		if b.Len() > 0 {
			out = append(out, Span{b.String(), s.Colour})
		}
		break
	}
	return out
}

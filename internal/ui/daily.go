// Deprecated: nothing in the app reaches this file. The daily table was the
// one view that stood in the chart's place, toggled with d; the key and the
// mode behind it are gone, and dailyView has no caller. It is kept for the
// summarising it does — a day's low and high, its rain total, its strongest
// gust and a sparkline of its curve — in case the table comes back as a range
// of its own. Delete it if it does not.

package ui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/olli-io/forecastui/internal/fmi"
	"github.com/olli-io/forecastui/internal/render"
)

// day is one calendar day's summary.
type day struct {
	date       time.Time
	min, max   float64
	haveTemp   bool
	rain       float64
	gust       float64
	sym        int
	sparkTemps []fmi.Val
}

// spark renders a temperature trace using the same eight-step block ramp the
// rest of the UI uses for magnitude.
var sparkRunes = [...]rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// summarise groups hours into days. It works from the hourly data rather than
// the chart columns, so the table is unaffected by 3 h aggregation.
func summarise(hours []fmi.Hour) []day {
	var days []day
	for _, h := range hours {
		at := h.Time.Local()
		y, m, d := at.Date()
		date := time.Date(y, m, d, 0, 0, 0, 0, at.Location())

		if len(days) == 0 || !days[len(days)-1].date.Equal(date) {
			days = append(days, day{date: date})
		}
		cur := &days[len(days)-1]

		if h.Temp.OK {
			if !cur.haveTemp {
				cur.min, cur.max, cur.haveTemp = h.Temp.V, h.Temp.V, true
			}
			cur.min = math.Min(cur.min, h.Temp.V)
			cur.max = math.Max(cur.max, h.Temp.V)
		}
		cur.rain += h.Rain.Or(0)
		if h.Gust.OK {
			cur.gust = math.Max(cur.gust, h.Gust.V)
		}
		// The midday symbol stands for the day: it is the one people picture.
		if at.Hour() == 12 && h.Sym != 0 {
			cur.sym = h.Sym
		} else if cur.sym == 0 {
			cur.sym = h.Sym
		}
		cur.sparkTemps = append(cur.sparkTemps, h.Temp)
	}
	return days
}

// Deprecated: unreferenced. See the note at the top of the file.
func (a *App) dailyView() string {
	days := summarise(a.hours)
	if len(days) == 0 {
		return a.statusOnly()
	}

	// One scale across all days, so the sparklines are comparable row to row.
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, d := range days {
		if d.haveTemp {
			lo, hi = math.Min(lo, d.min), math.Max(hi, d.max)
		}
	}
	if math.IsInf(lo, 1) {
		lo, hi = 0, 1
	}
	if hi <= lo {
		hi = lo + 1
	}

	lines := []render.Line{
		Header(a.place.Label(), a.cols, a.span.Slots),
		blank(),
		{{Text: "  day            low     high      rain      gust   sky", Colour: render.Grey}},
	}

	for _, d := range days {
		row := render.Line{
			{Text: fmt.Sprintf("  %-13s", d.date.Format("Mon 2 Jan")), Colour: render.FG},
		}
		if d.haveTemp {
			row = append(row,
				render.Span{Text: fmt.Sprintf("%6.1f°", d.min), Colour: render.TempColour(d.min, true)},
				render.Span{Text: fmt.Sprintf("%8.1f°", d.max), Colour: render.TempColour(d.max, true)})
		} else {
			row = append(row, render.Span{Text: fmt.Sprintf("%7s%9s", "—", "—"), Colour: render.Grey})
		}

		rain, rainCol := fmt.Sprintf("%10s", "—"), render.Grey
		if d.rain > 0.05 {
			rain, rainCol = fmt.Sprintf("%8.1fmm", d.rain), render.Blue
		}
		row = append(row, render.Span{Text: rain, Colour: rainCol})

		gust, gustCol := fmt.Sprintf("%10s", "—"), render.Grey
		if d.gust > 0 {
			gust, gustCol = fmt.Sprintf("%7.0fm/s", d.gust), render.WindColour(d.gust)
		}
		row = append(row, render.Span{Text: gust, Colour: gustCol})

		// A whole day's row always gets the daytime glyph: a moon would be
		// claiming something about an hour this line does not stand for.
		s := fmi.Describe(d.sym, false)
		row = append(row,
			render.Span{Text: "   " + string(s.Glyph) + " ", Colour: render.FG},
			render.Span{Text: fmt.Sprintf("%-22s", s.Desc), Colour: render.Grey},
			render.Span{Text: spark(d.sparkTemps, lo, hi), Colour: render.FG})
		lines = append(lines, row)
	}

	body := Paint(lines, true, a.width)
	return body + "\n" + a.pad(len(lines)) + a.footer()
}

// spark draws a day's temperature trace. Gaps render as a space so a missing
// hour is visible as a hole rather than a dip to the floor.
func spark(vals []fmi.Val, lo, hi float64) string {
	var b strings.Builder
	for _, v := range vals {
		if !v.OK {
			b.WriteRune(' ')
			continue
		}
		i := int((v.V - lo) / (hi - lo) * float64(len(sparkRunes)-1))
		b.WriteRune(sparkRunes[clamp(i, 0, len(sparkRunes)-1)])
	}
	return b.String()
}

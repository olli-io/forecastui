// Package render turns forecast hours into braille charts. It emits coloured
// spans and has no terminal or Bubble Tea dependency; the UI layer paints them.
package render

import (
	"math"
	"time"

	"github.com/olli-io/forecastui/internal/fmi"
	"github.com/olli-io/forecastui/internal/sun"
)

// Column is one bar-pair position on the chart.
type Column struct {
	At     time.Time // local time
	Temp   fmi.Val
	Feels  fmi.Val // apparent temperature
	Rain   float64 // mm/h; absent reads as zero
	Wind   fmi.Val
	Gust   fmi.Val
	Dir    fmi.Val
	POP    fmi.Val
	Cloud  fmi.Val
	Sym    int
	NewDay bool // first column of a local calendar day
	Night  bool // the sun is below the horizon at this hour, here
}

// slotStep is how many hours one column of the aggregated view stands for.
const slotStep = 3

// Columns converts forecast hours into chart columns in local time. With slots
// set, every third hour is kept as a centred 3 h rolling mean. The coordinates
// only decide which hours are drawn as night.
func Columns(hours []fmi.Hour, slots bool, lat, lon float64) []Column {
	cols := make([]Column, 0, len(hours))
	var prevDay time.Time
	hasPrev := false

	add := func(c Column) {
		c.Night = sun.Night(c.At, lat, lon)
		y, m, d := c.At.Date()
		today := time.Date(y, m, d, 0, 0, 0, 0, c.At.Location())
		c.NewDay = !hasPrev || !today.Equal(prevDay)
		prevDay, hasPrev = today, true
		cols = append(cols, c)
	}

	for i, h := range hours {
		at := h.Time.Local()
		if !slots {
			c := fromHour(h, at)
			add(c)
			continue
		}
		// Steps sit on wall-clock multiples of three, not on the hour the app
		// was opened, so the column under 12 is always noon.
		if at.Hour()%slotStep != 0 {
			continue
		}
		add(mean(hours, i, at))
	}

	// In hourly mode the first column is not a day boundary.
	if len(cols) > 0 {
		cols[0].NewDay = slots
	}
	return cols
}

func fromHour(h fmi.Hour, at time.Time) Column {
	return Column{
		At: at, Temp: h.Temp, Feels: gFeels(h), Rain: h.Rain.Or(0),
		Wind: h.Wind, Gust: h.Gust, Dir: h.Dir,
		POP: h.POP, Cloud: h.Cloud, Sym: h.Sym,
	}
}

// mean builds a column from the centred three-hour window around index i.
func mean(hours []fmi.Hour, i int, at time.Time) Column {
	c := Column{At: at, Sym: hours[i].Sym}
	var tSum, tN float64
	var rSum, n float64
	avg := func(get func(fmi.Hour) fmi.Val) fmi.Val {
		var s, k float64
		for j := i - 1; j <= i+1; j++ {
			if j < 0 || j >= len(hours) {
				continue
			}
			if v := get(hours[j]); v.OK {
				s, k = s+v.V, k+1
			}
		}
		if k == 0 {
			return fmi.Val{}
		}
		return fmi.Val{V: s / k, OK: true}
	}

	for j := i - 1; j <= i+1; j++ {
		if j < 0 || j >= len(hours) {
			continue
		}
		if v := hours[j].Temp; v.OK {
			tSum, tN = tSum+v.V, tN+1
		}
		// Rain averages over the window size, not present values: a gap is dry.
		rSum, n = rSum+hours[j].Rain.Or(0), n+1
	}
	if tN > 0 {
		c.Temp = fmi.Val{V: tSum / tN, OK: true}
	}
	if n > 0 {
		c.Rain = rSum / n
	}
	c.Wind, c.Gust, c.Cloud, c.POP = avg(gWind), avg(gGust), avg(gCloud), avg(gPOP)
	c.Feels = avg(gFeels)
	c.Dir = hours[i].Dir // directions do not average across a wrap
	return c
}

func gWind(h fmi.Hour) fmi.Val  { return h.Wind }
func gGust(h fmi.Hour) fmi.Val  { return h.Gust }
func gCloud(h fmi.Hour) fmi.Val { return h.Cloud }
func gPOP(h fmi.Hour) fmi.Val   { return h.POP }

// gFeels derives the apparent temperature; the service does not publish one.
func gFeels(h fmi.Hour) fmi.Val { return fmi.FeelsLike(h.Temp, h.Wind, h.Hum) }

// Scale is the vertical mapping shared by every column, computed once over the
// whole forecast so scrolling sideways never makes the bars jump.
type Scale struct {
	Hi, Lo  float64 // observed temperature bounds, feels-like included
	Floor   float64 // bottom of the temperature axis
	RainMax float64
	WindMax float64
	Empty   bool // no known temperatures at all
}

// NewScale derives the axis bounds from every column.
func NewScale(cols []Column) Scale {
	var s Scale
	s.Empty = true
	for _, c := range cols {
		if c.Temp.OK {
			if s.Empty {
				s.Hi, s.Lo, s.Empty = c.Temp.V, c.Temp.V, false
			}
			s.Hi = math.Max(s.Hi, c.Temp.V)
			s.Lo = math.Min(s.Lo, c.Temp.V)
		}
		// The apparent temperature shares the axis, so it has to fit on it.
		if c.Feels.OK && !s.Empty {
			s.Hi = math.Max(s.Hi, c.Feels.V)
			s.Lo = math.Min(s.Lo, c.Feels.V)
		}
		s.RainMax = math.Max(s.RainMax, c.Rain)
		if c.Wind.OK {
			s.WindMax = math.Max(s.WindMax, c.Wind.V)
		}
		if c.Gust.OK {
			s.WindMax = math.Max(s.WindMax, c.Gust.V)
		}
	}
	if s.Hi == s.Lo {
		s.Hi = s.Lo + 1.0
	}
	// A margin below the coldest reading, so the lowest bar is a visible stub.
	s.Floor = s.Lo - 0.10*(s.Hi-s.Lo) - 0.2
	return s
}

// pos maps a temperature onto the dot grid.
func (s Scale) pos(v float64, dots int) float64 {
	return (v - s.Floor) / (s.Hi - s.Floor) * float64(dots)
}

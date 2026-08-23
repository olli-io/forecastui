package render

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/olli-io/forecastui/internal/fmi"
)

func TestArrowPointsWhereTheWindBlows(t *testing.T) {
	// FMI reports the direction wind comes from, so a northerly blows south.
	for _, tc := range []struct {
		from  float64
		arrow rune
		name  string
	}{
		{0, '↓', "N"}, {90, '←', "E"}, {180, '↑', "S"}, {270, '→', "W"},
		{45, '↙', "NE"}, {315, '↘', "NW"},
	} {
		if got := Arrow(tc.from); got != tc.arrow {
			t.Errorf("wind from %.0f°: arrow %c, want %c", tc.from, got, tc.arrow)
		}
		if got := Compass(tc.from); got != tc.name {
			t.Errorf("wind from %.0f°: %q, want %q", tc.from, got, tc.name)
		}
	}
}

func TestArrowWrapsAroundNorth(t *testing.T) {
	if Arrow(359) != Arrow(1) {
		t.Error("bearings either side of north should agree")
	}
	if Compass(359) != "N" {
		t.Errorf("359° should read as N, got %q", Compass(359))
	}
}

// Every glyph the panels draw must be one cell wide, or the three-character
// column grid shears.
func TestPanelGlyphsAreSingleWidth(t *testing.T) {
	var glyphs []rune
	glyphs = append(glyphs, arrows[:]...)
	glyphs = append(glyphs, []rune(vert+corner+horiz+tick+dayEnd+rCorner+DownArrow)...)
	glyphs = append(glyphs, gArrow)
	for _, r := range glyphs {
		if w := lipgloss.Width(string(r)); w != 1 {
			t.Errorf("glyph %q (U+%04X) is %d cells wide", r, r, w)
		}
	}
}

func TestPanelsAlignWithTheChart(t *testing.T) {
	cols := demoColumns(24)
	sc := NewScale(cols)
	o := Opts{Count: 12}

	// The hour label row under the chart is its true content width.
	axis := Axis(cols, sc, o)
	want := len([]rune(axis[len(axis)-1].Plain()))

	for name, lines := range map[string][]Line{
		"speeds": {WindSpeeds(cols, o)},
		"rates":  {RainLabels(cols, sc, o)},
		"dirs":   {WindDirs(cols, o)},
		"rain":   Rain(cols, sc, o),
		"wind":   Wind(cols, sc, o),
		"sky":    Sky(cols, o),
	} {
		if lines == nil {
			t.Fatalf("%s panel rendered nothing", name)
		}
		for i, l := range lines {
			if got := len([]rune(l.Plain())); got > want+2 {
				t.Errorf("%s line %d is %d wide, chart is %d", name, i, got, want)
			}
		}
	}
}

func TestWindPanelNeedsWind(t *testing.T) {
	cols := []Column{{At: time.Now(), Temp: fmi.Val{V: 5, OK: true}}}
	if Wind(cols, NewScale(cols), Opts{}) != nil {
		t.Error("a forecast with no wind should draw no wind panel")
	}
}

func TestRainPanelAlwaysDrawn(t *testing.T) {
	cols := []Column{{At: time.Now(), Temp: fmi.Val{V: 5, OK: true}}}
	dry := Rain(cols, NewScale(cols), Opts{})
	if dry == nil {
		t.Fatal("a dry forecast should still draw the rain panel")
	}
	if RainLabels(cols, NewScale(cols), Opts{}) == nil {
		t.Error("a dry forecast should still draw the rate row")
	}
	for i, l := range dry {
		if strings.ContainsAny(l.Plain(), "⣿⣀⡀⠄") {
			t.Errorf("dry rain line %d has bars: %q", i, l.Plain())
		}
	}
	cols[0].Rain = 0.4
	wet := Rain(cols, NewScale(cols), Opts{})
	if wet == nil {
		t.Fatal("a wet forecast should draw one")
	}
	if len(wet) != len(dry) {
		t.Errorf("wet panel is %d lines, dry one %d", len(wet), len(dry))
	}
}

// demoColumns builds a deterministic set of columns for layout assertions.
func demoColumns(n int) []Column {
	base := time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC)
	cols := make([]Column, n)
	for i := range cols {
		f := float64(i)
		cols[i] = Column{
			At:     base.Add(time.Duration(i) * time.Hour),
			Temp:   fmi.Val{V: 10 + f/3, OK: true},
			Rain:   f / 20,
			Wind:   fmi.Val{V: 3 + f/8, OK: true},
			Gust:   fmi.Val{V: 6 + f/6, OK: true},
			Dir:    fmi.Val{V: f * 15, OK: true},
			Cloud:  fmi.Val{V: f * 4, OK: true},
			Sym:    1,
			NewDay: i == 0,
		}
	}
	return cols
}

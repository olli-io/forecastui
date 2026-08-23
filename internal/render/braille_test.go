package render

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/olli-io/forecastui/internal/fmi"
)

var update = flag.Bool("update", false, "rewrite golden files")

// The fixture is a Turku forecast; the coordinates decide which hours the sky
// row draws as night.
const turkuLat, turkuLon = 60.4518, 22.2666

func fixture(t *testing.T) []fmi.Hour {
	t.Helper()
	body, err := os.ReadFile("../fmi/testdata/turku48.json")
	if err != nil {
		t.Fatal(err)
	}
	hours, err := fmi.Parse(body)
	if err != nil {
		t.Fatal(err)
	}
	return hours
}

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".txt")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run: go test ./internal/render -update)", err)
	}
	if string(want) != got+"\n" {
		t.Errorf("golden %s mismatch\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

func TestChartGolden(t *testing.T) {
	// Rendering is time-zone sensitive through Column's local conversion.
	t.Setenv("TZ", "Europe/Helsinki")
	time.Local = mustLoad(t, "Europe/Helsinki")

	hours := fixture(t)
	for _, tc := range []struct {
		name  string
		slots bool
		width int
	}{
		{"hourly-w80", false, 80},
		{"hourly-w120", false, 120},
		{"hourly-w40", false, 40},
		{"slots-w80", true, 80},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cols := Columns(hours, tc.slots, turkuLat, turkuLon)
			sc := NewScale(cols)
			o := Opts{Count: Fits(tc.width), Slots: tc.slots, Cursor: -1}
			lines := append(Chart(cols, sc, o), Rule(cols, o, fmt.Sprintf("%.1f°C", sc.Lo)))
			rain := o
			rain.CellH = 4
			lines = append(lines, Rain(cols, sc, rain)...)
			lines = append(lines, Rule(cols, o, "0"))
			lines = append(lines, HourLabels(cols, o)...)
			lines = append(lines, TempLabels(cols, o))
			lines = append(lines, RainLabels(cols, sc, o))
			golden(t, tc.name, Plain(lines))
		})
	}
}

func mustLoad(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}
	return loc
}

func TestChartFitsWidth(t *testing.T) {
	t.Setenv("TZ", "Europe/Helsinki")
	hours := fixture(t)
	cols := Columns(hours, false, turkuLat, turkuLon)
	sc := NewScale(cols)
	for _, w := range []int{40, 60, 80, 100, 120, 200} {
		o := Opts{Count: Fits(w)}
		lines := append(Chart(cols, sc, o), Axis(cols, sc, o)...)
		lines = append(lines, Rain(cols, sc, Opts{Count: Fits(w), CellH: 4})...)
		lines = append(lines, Wind(cols, sc, Opts{Count: Fits(w), CellH: 4})...)
		for i, l := range lines {
			if got := len([]rune(l.Plain())); got > w {
				t.Errorf("width %d: line %d is %d wide", w, i, got)
			}
		}
	}
}

func TestChartWindowDoesNotRescale(t *testing.T) {
	t.Setenv("TZ", "Europe/Helsinki")
	hours := fixture(t)
	cols := Columns(hours, false, turkuLat, turkuLon)
	sc := NewScale(cols)
	// The axis labels are the scale made visible; panning must not move them.
	first := Chart(cols, sc, Opts{Start: 0, Count: 12})
	later := Chart(cols, sc, Opts{Start: 20, Count: 12})
	if first[0][0].Text != later[0][0].Text {
		t.Errorf("top axis label changed while scrolling: %q vs %q",
			first[0][0].Text, later[0][0].Text)
	}
}

func TestCellBitsMatchBarHeight(t *testing.T) {
	// A full-height bar fills every dot of every cell.
	const cellH = 6
	for row := 0; row < cellH; row++ {
		if got := cell(cellH*4, row, cellH); got != 0xFF {
			t.Errorf("row %d of a full bar: %#x, want 0xff", row, got)
		}
	}
	// An empty bar lights nothing.
	for row := 0; row < cellH; row++ {
		if got := cell(0, row, cellH); got != 0 {
			t.Errorf("row %d of an empty bar: %#x, want 0", row, got)
		}
	}
	// A one-dot bar lights only the bottom dot row of the bottom cell.
	if got := cell(1, cellH-1, cellH); got != leftDots[3]|rightDots[3] {
		t.Errorf("one-dot bar: %#x", got)
	}
	if got := cell(1, cellH-2, cellH); got != 0 {
		t.Errorf("one-dot bar leaked into the cell above: %#x", got)
	}
}

func TestScaleEdgeCases(t *testing.T) {
	at := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	t.Run("flat temperature still spans a degree", func(t *testing.T) {
		sc := NewScale([]Column{
			{At: at, Temp: fmi.Val{V: 5, OK: true}},
			{At: at, Temp: fmi.Val{V: 5, OK: true}},
		})
		if sc.Hi <= sc.Lo {
			t.Fatalf("hi %v must exceed lo %v", sc.Hi, sc.Lo)
		}
	})
	t.Run("no temperatures at all", func(t *testing.T) {
		cols := []Column{{At: at}, {At: at}}
		sc := NewScale(cols)
		if !sc.Empty {
			t.Fatal("scale should report empty")
		}
		if lines := Chart(cols, sc, Opts{}); len(lines) == 0 {
			t.Fatal("chart should still draw its axes")
		}
	})
	t.Run("single column", func(t *testing.T) {
		cols := []Column{{At: at, Temp: fmi.Val{V: 5, OK: true}}}
		if lines := Chart(cols, NewScale(cols), Opts{}); len(lines) == 0 {
			t.Fatal("a single column should render")
		}
	})
	t.Run("no columns", func(t *testing.T) {
		if lines := Chart(nil, NewScale(nil), Opts{}); lines != nil {
			t.Fatal("no columns should render nothing")
		}
	})
	t.Run("dry forecast", func(t *testing.T) {
		// Rain has its own panel; the chart must not mention it.
		cols := []Column{{At: at, Temp: fmi.Val{V: 5, OK: true}}}
		for _, l := range Chart(cols, NewScale(cols), Opts{}) {
			if contains(l.Plain(), "mm") {
				t.Errorf("the temperature chart still carries a rain axis: %q", l.Plain())
			}
		}
	})
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/olli-io/forecastui/internal/fmi"
	"github.com/olli-io/forecastui/internal/geo"
	"github.com/olli-io/forecastui/internal/render"
)

// onceHours is a forecast that starts on the hour the test runs in, the way
// the fetch does, so the column the dump picks out is a known one.
func onceHours(n int) []fmi.Hour {
	hours := demoHours(n)
	base := time.Now().UTC().Truncate(time.Hour)
	for i := range hours {
		hours[i].Time = base.Add(time.Duration(i) * time.Hour)
	}
	return hours
}

func onceOutput(t *testing.T, hours []fmi.Hour, span Span, width int) []string {
	t.Helper()
	t.Setenv("NO_COLOR", "1")
	out, err := Once(geo.Place{Name: "Turku", Lat: 60.4518, Lon: 22.2666},
		hours, span, width, true)
	if err != nil {
		t.Fatalf("Once: %v", err)
	}
	return strings.Split(out, "\n")
}

// The dump has no cursor to move, so it stands on the hour it was printed in:
// the first column of a forecast fetched from now.
func TestOnceMarksTheCurrentHour(t *testing.T) {
	lines := onceOutput(t, onceHours(24), Span{Hours: 24}, 120)

	var foot, top string
	for _, l := range lines {
		if strings.Contains(l, "▲") {
			foot = l
		}
		if strings.Contains(l, "▼") {
			top = l
		}
	}
	if foot == "" || top == "" {
		t.Fatalf("no cursor frame or detail box in the dump:\n%s", strings.Join(lines, "\n"))
	}
	// The detail box's arrow answers the frame's: both stand in the column
	// the reading belongs to, or the box stops pointing at the hour.
	if got, want := column(top, "▼"), column(foot, "▲"); got != want {
		t.Errorf("box arrow at %d, frame arrow at %d\n%s\n%s", got, want, foot, top)
	}
	// The frame stands on the first column, so the arrow is over the first
	// bar — at the left edge of the chart, not out in the middle of it.
	if want := render.AxisW; column(foot, "▲") != want {
		t.Errorf("frame arrow at %d, want the first column at %d\n%s",
			column(foot, "▲"), want, foot)
	}
	// And the box under it reads out that hour.
	now := time.Now().UTC().Truncate(time.Hour).Local().Format("Mon 02 15:04")
	if !strings.Contains(strings.Join(lines, "\n"), now) {
		t.Errorf("detail box does not name the current hour %q:\n%s",
			now, strings.Join(lines, "\n"))
	}
}

// A forecast whose hours have all gone by — a stale cache, or a clock that has
// moved — puts the cursor on the last column rather than off the end of them.
func TestNowColumnStandsOnTheLatestPastHour(t *testing.T) {
	base := time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC)
	cols := render.Columns(demoHours(6), false, 60.4518, 22.2666)

	for _, tc := range []struct {
		name string
		now  time.Time
		want int
	}{
		{"before the forecast starts", base.Add(-time.Hour), 0},
		{"on the first hour", base, 0},
		{"part way through", base.Add(3*time.Hour + 30*time.Minute), 3},
		{"after it ends", base.Add(99 * time.Hour), len(cols) - 1},
	} {
		if got := nowColumn(cols, tc.now); got != tc.want {
			t.Errorf("%s: column %d, want %d", tc.name, got, tc.want)
		}
	}
}

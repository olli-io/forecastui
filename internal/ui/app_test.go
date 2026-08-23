package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/olli-io/forecastui/internal/fmi"
	"github.com/olli-io/forecastui/internal/geo"
	"github.com/olli-io/forecastui/internal/render"
)

func demoHours(n int) []fmi.Hour {
	base := time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC)
	hours := make([]fmi.Hour, n)
	for i := range hours {
		f := float64(i)
		hours[i] = fmi.Hour{
			Time:  base.Add(time.Duration(i) * time.Hour),
			Temp:  fmi.Val{V: 10 + f/4, OK: true},
			Wind:  fmi.Val{V: 4, OK: true},
			Gust:  fmi.Val{V: 8, OK: true},
			Dir:   fmi.Val{V: 90, OK: true},
			Rain:  fmi.Val{V: f / 30, OK: true},
			POP:   fmi.Val{V: f, OK: true},
			Cloud: fmi.Val{V: 50, OK: true},
			Sym:   3,
		}
	}
	return hours
}

func newTestApp(t *testing.T, w, h, hours int) *App {
	t.Helper()
	a := New(geo.Place{Name: "Turku", Lat: 60.4518, Lon: 22.2666},
		Span{Hours: hours}, &geo.Config{}, true).(*App)
	a.width, a.height = w, h
	a.loading = false
	a.setHours(demoHours(hours))
	return a
}

func press(a *App, key string) {
	a.Update(tea.KeyPressMsg{Code: keyCode(key), Text: key})
}

// keyCode maps the handful of names the tests use onto rune codes.
func keyCode(key string) rune {
	switch key {
	case "right":
		return tea.KeyRight
	case "left":
		return tea.KeyLeft
	case "end":
		return tea.KeyEnd
	case "home":
		return tea.KeyHome
	}
	return rune(key[0])
}

// The view must never emit a line wider than the terminal: Bubble Tea would
// wrap it and the whole braille grid would shear.
func TestViewNeverExceedsTerminalWidth(t *testing.T) {
	for _, size := range [][2]int{{40, 12}, {60, 20}, {80, 24}, {100, 16}, {200, 50}} {
		w, h := size[0], size[1]
		a := newTestApp(t, w, h, 48)
		for i, line := range strings.Split(a.render(), "\n") {
			if got := lipglossWidth(line); got > w {
				t.Errorf("chart at %dx%d: line %d is %d wide\n%q", w, h, i, got, line)
			}
		}
	}
}

func TestViewFitsTerminalHeight(t *testing.T) {
	for _, size := range [][2]int{{40, 12}, {80, 24}, {120, 40}} {
		w, h := size[0], size[1]
		a := newTestApp(t, w, h, 48)
		if got := len(strings.Split(a.render(), "\n")); got > h {
			t.Errorf("at %dx%d the view is %d lines tall", w, h, got)
		}
	}
}

func TestCursorStaysInRange(t *testing.T) {
	a := newTestApp(t, 80, 24, 48)
	for i := 0; i < 200; i++ {
		press(a, "right")
	}
	if a.cursor != len(a.cols)-1 {
		t.Errorf("cursor ran to %d, want %d", a.cursor, len(a.cols)-1)
	}
	for i := 0; i < 200; i++ {
		press(a, "left")
	}
	if a.cursor != 0 {
		t.Errorf("cursor ran to %d, want 0", a.cursor)
	}
}

// Moving right past the edge must scroll the chart, not page it: the week view
// is one continuous strip.
func TestCursorScrollsTheViewport(t *testing.T) {
	a := newTestApp(t, 80, 24, 48)
	n := a.visible()
	if n >= len(a.cols) {
		t.Fatalf("test needs more columns than fit: %d visible of %d", n, len(a.cols))
	}
	for i := 0; i < n; i++ {
		press(a, "right")
	}
	if a.scroll == 0 {
		t.Fatal("viewport did not scroll when the cursor left it")
	}
	if a.cursor < a.scroll || a.cursor >= a.scroll+n {
		t.Errorf("cursor %d outside viewport [%d,%d)", a.cursor, a.scroll, a.scroll+n)
	}
	// Scrolling by one keeps all but one column on screen, unlike paging.
	if a.scroll != a.cursor-n+1 {
		t.Errorf("scroll %d looks like paging, want %d", a.scroll, a.cursor-n+1)
	}
}

func TestScrollStaysWithinBounds(t *testing.T) {
	a := newTestApp(t, 80, 24, 48)
	press(a, "end")
	if a.scroll+a.visible() > len(a.cols) {
		t.Errorf("scroll %d + %d visible overruns %d columns",
			a.scroll, a.visible(), len(a.cols))
	}
	press(a, "home")
	if a.scroll != 0 {
		t.Errorf("home should scroll back to the start, got %d", a.scroll)
	}
}

func TestNarrowTerminalStillShowsAColumn(t *testing.T) {
	a := newTestApp(t, 10, 8, 24)
	if a.visible() < 1 {
		t.Fatal("a very narrow terminal must still show one column")
	}
	if a.render() == "" {
		t.Fatal("a very narrow terminal rendered nothing")
	}
}

func TestRangeCycleIsARoundTrip(t *testing.T) {
	a := newTestApp(t, 80, 24, 48)
	start := a.span
	for range ranges {
		a.span = nextRange(a.span)
	}
	if a.span != start {
		t.Errorf("cycling every range returned %+v, want %+v", a.span, start)
	}
	if prevRange(nextRange(start)) != start {
		t.Error("prev should undo next")
	}
}

func TestEmptyForecastRendersAMessageNotAPanic(t *testing.T) {
	a := New(geo.Place{Name: "Turku"}, Span{Hours: 24}, &geo.Config{}, true).(*App)
	a.width, a.height = 80, 24
	out := a.render()
	if !strings.Contains(out, "Turku") {
		t.Errorf("empty state should still name the place:\n%s", out)
	}
}

func TestStaleCacheIsMarked(t *testing.T) {
	a := newTestApp(t, 120, 30, 24)
	a.stale = true
	a.fetched = time.Now().Add(-90 * time.Minute)
	if got := render.Plain(a.header()); !strings.Contains(got, "cached") {
		t.Errorf("stale data should say so: %q", got)
	}
}

func TestDetailTracksTheCursor(t *testing.T) {
	a := newTestApp(t, 120, 30, 48)
	press(a, "right")
	press(a, "right")
	want := a.cols[2].At.Format("15:04")
	if got := render.Plain(a.detail()); !strings.Contains(got, want) {
		t.Errorf("detail should describe %s:\n%s", want, got)
	}
}

// column is where sub starts in s counted in terminal cells, not bytes: every
// glyph the chart draws is one cell wide, so runes index columns.
func column(s, sub string) int {
	i := strings.Index(s, sub)
	if i < 0 {
		return -1
	}
	return len([]rune(s[:i]))
}

// The detail box's arrow answers the cursor frame's: both must stand in the
// same column, or the box stops pointing at the hour it describes.
func TestDetailArrowSitsUnderTheCursors(t *testing.T) {
	for _, w := range []int{70, 92, 120} {
		a := newTestApp(t, w, 40, 48)
		for _, cur := range []int{0, 3, a.visible() - 1} {
			a.cursor = cur
			a.clampScroll()
			var foot string
			for _, l := range render.Cursor(nil, a.cols, render.Opts{
				Start: a.scroll, Count: a.visible(), Cursor: a.cursor,
			}) {
				if strings.Contains(l.Plain(), "▲") {
					foot = l.Plain()
				}
			}
			top := a.detail()[0].Plain()
			if got, want := column(top, "▼"), column(foot, "▲"); got != want {
				t.Errorf("width %d cursor %d: box arrow at %d, frame arrow at %d\n%s\n%s",
					w, cur, got, want, foot, top)
			}
		}
	}
}

// The shortcut list stands two columns inside the boxes' own edge.
func TestFooterLinesUpWithTheBoxes(t *testing.T) {
	for _, w := range []int{40, 60, 70, 92, 100, 160} {
		a := newTestApp(t, w, 30, 48)
		box := a.detail()[0].Plain()
		foot := a.footer()
		keys := strings.TrimLeft(stripANSI(foot), " ")
		if got, want := lipglossWidth(stripANSI(foot))-lipglossWidth(keys),
			column(box, "┌")+2; got != want {
			t.Errorf("width %d: footer indented %d, boxes at %d\n%s", w, got, want, foot)
		}
	}
}

// stripANSI drops the colour escapes Paint adds, leaving the cells themselves.
func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func TestBoxesLineUpWithTheChart(t *testing.T) {
	for _, w := range []int{70, 80, 100, 120, 160} {
		a := newTestApp(t, w, 40, 48)
		rule := render.Rule(a.cols, render.Opts{Start: a.scroll, Count: a.visible()}, "").Plain()
		// A box stands one column in from the screen's left edge and closes on
		// the chart's right wall: the chart hangs inside it, rather than the
		// box standing in the chart's own walls.
		for name, box := range map[string]string{
			"detail": a.detail()[0].Plain(), // each frame's top edge
			"header": a.header()[0].Plain(),
		} {
			if got, want := lipglossWidth(box), lipglossWidth(rule); got != want {
				t.Errorf("width %d: %s box ends at %d, chart rule at %d\n%s\n%s",
					w, name, got, want, rule, box)
			}
			if got := strings.Index(box, "┌"); got != boxIndent {
				t.Errorf("width %d: %s box starts at %d, want %d\n%s",
					w, name, got, boxIndent, box)
			}
		}
	}
}

func TestDetailFieldsKeepTheirWidth(t *testing.T) {
	a := newTestApp(t, 160, 30, 48)
	first := render.Plain(a.detail())
	// A gap in the forecast must not shift the fields around it.
	h := demoHours(48)
	for i := range h {
		h[i].Gust, h[i].POP, h[i].Cloud = fmi.Val{}, fmi.Val{}, fmi.Val{}
	}
	a.setHours(h)
	second := render.Plain(a.detail())
	if len(strings.Split(first, "\n")) != len(strings.Split(second, "\n")) {
		t.Errorf("missing readings changed the box shape:\n%s\n%s", first, second)
	}
	if i, j := strings.Index(first, "m/s"), strings.Index(second, "m/s"); i != j {
		t.Errorf("wind moved from column %d to %d:\n%s\n%s", i, j, first, second)
	}
}

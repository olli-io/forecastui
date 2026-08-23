package ui

import (
	"strings"
	"testing"

	"github.com/olli-io/forecastui/internal/geo"
)

func places(names ...string) []geo.Place {
	out := make([]geo.Place, len(names))
	for i, n := range names {
		// Coordinates only have to differ; Same is what tells two apart.
		out[i] = geo.Place{Name: n, Lat: 60 + float64(i), Lon: 20 + float64(i)}
	}
	return out
}

// withResults opens the search window on a canned set of results, so the tests
// never touch the network.
func withResults(t *testing.T, a *App, names ...string) {
	t.Helper()
	a.openSearch()
	a.search.input.SetValue("q")
	a.search.results = places(names...)
	a.search.selected = 0
}

// A floating window is drawn over the chart, not in place of it: the header
// box and the footer must both survive underneath.
func TestSearchWindowFloatsOverTheChart(t *testing.T) {
	a := newTestApp(t, 100, 30, 48)
	withResults(t, a, "Turku, Finland")
	out := stripANSI(a.render())
	for _, want := range []string{"Find a place", "Turku, Finland", "day", "esc close"} {
		if !strings.Contains(out, want) {
			t.Errorf("the overlaid view is missing %q:\n%s", want, out)
		}
	}
}

func TestFavouritesWindowFloatsOverTheChart(t *testing.T) {
	a := newTestApp(t, 100, 30, 48)
	a.cfg.Favourites = places("Tromsø", "Kiruna")
	a.openFavourites()
	out := stripANSI(a.render())
	for _, want := range []string{"Favourites", "Tromsø", "Kiruna", "day"} {
		if !strings.Contains(out, want) {
			t.Errorf("the overlaid view is missing %q:\n%s", want, out)
		}
	}
}

// The compositor clips to the screen, so a window can never shear the grid
// under it however long a place name is or however small the terminal.
func TestFloatingWindowsFitTheTerminal(t *testing.T) {
	for _, size := range [][2]int{{20, 8}, {40, 12}, {60, 20}, {80, 24}, {100, 16}, {200, 50}} {
		w, h := size[0], size[1]
		a := newTestApp(t, w, h, 48)
		a.cfg.Favourites = places(
			"Ylä-Savon seutukunta, Pohjois-Savo, Itä-Suomen aluehallintovirasto, Finland",
			"Kiruna")
		for _, open := range []struct {
			name string
			do   func()
		}{
			{"search", func() { withResults(t, a, "A very long place name indeed, somewhere") }},
			{"favourites", a.openFavourites},
		} {
			open.do()
			lines := strings.Split(a.render(), "\n")
			if len(lines) > h {
				t.Errorf("%s at %dx%d is %d lines tall", open.name, w, h, len(lines))
			}
			for i, line := range lines {
				if got := lipglossWidth(line); got > w {
					t.Errorf("%s at %dx%d: line %d is %d wide\n%q",
						open.name, w, h, i, got, line)
				}
			}
		}
	}
}

// The prompt stays put as results come and go: a window that walked up the
// screen while you typed would take the input line with it.
func TestSearchPromptDoesNotMoveAsResultsArrive(t *testing.T) {
	a := newTestApp(t, 100, 30, 48)
	a.openSearch()
	before := strings.Index(stripANSI(a.render()), "Find a place")
	a.search.results = places("One", "Two", "Three", "Four", "Five")
	after := strings.Index(stripANSI(a.render()), "Find a place")
	if before != after {
		t.Errorf("the window moved from %d to %d when results landed", before, after)
	}
}

// The point of the overhaul: several places can be starred without leaving
// the search window.
func TestSearchStarsMoreThanOnePlace(t *testing.T) {
	a := newTestApp(t, 100, 30, 48)
	withResults(t, a, "Turku", "Tromsø", "Kiruna")
	press(a, "tab")
	press(a, "down")
	press(a, "down")
	press(a, "tab")
	if got := len(a.cfg.Favourites); got != 2 {
		t.Fatalf("starred %d places, want 2: %+v", got, a.cfg.Favourites)
	}
	if a.mode != modeSearch {
		t.Error("starring a result should leave the window open")
	}
	if a.cfg.Favourites[0].Name != "Turku" || a.cfg.Favourites[1].Name != "Kiruna" {
		t.Errorf("starred %+v, want Turku and Kiruna", a.cfg.Favourites)
	}
	// Pressing it again on the same result takes the star off.
	press(a, "tab")
	if got := len(a.cfg.Favourites); got != 1 {
		t.Errorf("tab should unstar too, %d favourites left", got)
	}
}

// A starred result is marked as such the moment it is starred, so the list
// shows what has been saved so far.
func TestSearchMarksStarredResults(t *testing.T) {
	a := newTestApp(t, 100, 30, 48)
	withResults(t, a, "Turku", "Tromsø")
	if strings.Contains(stripANSI(a.render()), "★ Turku") {
		t.Fatal("nothing is starred yet")
	}
	press(a, "tab")
	if !strings.Contains(stripANSI(a.render()), "★ Turku") {
		t.Error("the starred result should carry its star")
	}
}

// Enter on a result switches location and closes the window.
func TestSearchEnterGoesToTheSelection(t *testing.T) {
	a := newTestApp(t, 100, 30, 48)
	withResults(t, a, "Turku", "Tromsø")
	press(a, "down")
	press(a, "enter")
	if a.mode != modeChart {
		t.Error("enter should close the window")
	}
	if a.place.Name != "Tromsø" {
		t.Errorf("went to %q, want Tromsø", a.place.Name)
	}
}

// f opens the list standing on the place already on screen, so enter is a
// no-op rather than a surprise jump.
func TestFavouritesOpenOnTheCurrentPlace(t *testing.T) {
	a := newTestApp(t, 100, 30, 48)
	a.cfg.Favourites = places("Tromsø", "Kiruna", "Turku")
	a.place = a.cfg.Favourites[2]
	press(a, "f")
	if a.mode != modeFav {
		t.Fatalf("f should open the favourites window, mode is %v", a.mode)
	}
	if a.fav.selected != 2 {
		t.Errorf("opened on entry %d, want 2", a.fav.selected)
	}
	press(a, "enter")
	if a.mode != modeChart || a.place.Name != "Turku" {
		t.Errorf("enter left mode %v at %q", a.mode, a.place.Name)
	}
}

func TestFavouritesWindowUnstars(t *testing.T) {
	a := newTestApp(t, 100, 30, 48)
	a.cfg.Favourites = places("Tromsø", "Kiruna", "Turku")
	a.openFavourites()
	press(a, "down")
	press(a, "x")
	if len(a.cfg.Favourites) != 2 || a.cfg.Favourites[1].Name != "Turku" {
		t.Errorf("removing the middle entry left %+v", a.cfg.Favourites)
	}
	// The selection must still point at something after the list shortens.
	press(a, "x")
	press(a, "x")
	if len(a.cfg.Favourites) != 0 {
		t.Errorf("emptying the list left %+v", a.cfg.Favourites)
	}
	if out := stripANSI(a.render()); !strings.Contains(out, "nothing saved yet") {
		t.Errorf("an empty list should say so:\n%s", out)
	}
}

// The list is where you notice a favourite is missing, so / opens the search
// from it without a trip back through the chart.
func TestSearchOpensFromTheFavouritesWindow(t *testing.T) {
	a := newTestApp(t, 100, 30, 48)
	a.cfg.Favourites = places("Tromsø")
	a.openFavourites()
	press(a, "/")
	if a.mode != modeSearch {
		t.Errorf("/ should open the search, mode is %v", a.mode)
	}
}

// Typing must reach the prompt, not the chart's own shortcuts.
func TestChartKeysDoNotFireWhileTheSearchIsOpen(t *testing.T) {
	a := newTestApp(t, 100, 30, 48)
	a.openSearch()
	span, cursor := a.span, a.cursor
	for _, k := range []string{"q", "d", "r", "l", "G"} {
		press(a, k)
	}
	if a.mode != modeSearch {
		t.Errorf("a chart key closed the search window, mode is %v", a.mode)
	}
	if a.span != span || a.cursor != cursor {
		t.Error("a chart key moved the chart while the search was open")
	}
	if got := a.search.input.Value(); got != "qdrlG" {
		t.Errorf("the prompt read %q, want %q", got, "qdrlG")
	}
}

func TestEscapeClosesAFloatingWindow(t *testing.T) {
	a := newTestApp(t, 100, 30, 48)
	for _, open := range []func(){a.openSearch, a.openFavourites} {
		open()
		press(a, "esc")
		if a.mode != modeChart {
			t.Errorf("esc left mode %v", a.mode)
		}
	}
}

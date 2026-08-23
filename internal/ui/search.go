package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/olli-io/forecastui/internal/geo"
	"github.com/olli-io/forecastui/internal/render"
)

// debounce is how long typing must pause before a lookup is sent.
const debounce = 400 * time.Millisecond

// markW is the room the cursor and the star take ahead of a name.
const markW = 5

// coordW fits "60.45, 22.27". Two decimals is about a kilometre, finer than
// the forecast grid, and it leaves the names room.
const coordW = 13

type searchState struct {
	ready    bool
	input    textinput.Model
	results  []geo.Place
	selected int
	seq      int // identifies the newest query, so stale replies can be dropped
	busy     bool
	err      error
}

type favState struct {
	selected int
}

type searchMsg struct {
	seq     int
	results []geo.Place
	err     error
}

type debounceMsg struct {
	seq   int
	query string
}

// openSearch raises the search prompt over the chart, empty and focused.
func (a *App) openSearch() {
	s := &a.search
	if !s.ready {
		s.input = textinput.New()
		s.input.Prompt = "/ "
		s.input.Placeholder = "Turku, Tromsø, Kiruna…"
		s.ready = true
	}
	s.input.Focus()
	s.input.SetValue("")
	s.results, s.selected, s.err, s.busy = nil, 0, nil, false
	a.mode = modeSearch
}

// openFavourites stands on whichever entry is already on screen, so opening it
// and pressing enter changes nothing.
func (a *App) openFavourites() {
	a.fav.selected = 0
	for i, f := range a.cfg.Favourites {
		if f.Same(a.place) {
			a.fav.selected = i
			break
		}
	}
	a.mode = modeFav
}

func (a *App) searchKey(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "esc", "ctrl+c":
		a.mode = modeChart
		return a, nil
	case "up", "ctrl+p":
		a.search.selected = clamp(a.search.selected-1, 0, max(0, len(a.search.results)-1))
		return a, nil
	case "down", "ctrl+n":
		a.search.selected = clamp(a.search.selected+1, 0, max(0, len(a.search.results)-1))
		return a, nil
	case "tab":
		// The window stays open, so several places can be saved in one visit.
		if p, ok := a.search.current(); ok {
			a.cfg.Toggle(p)
			_ = a.cfg.Save(appName) // a read-only config should not eat the key
		}
		return a, nil
	case "enter":
		p, ok := a.search.current()
		if !ok {
			return a, nil
		}
		a.mode = modeChart
		return a, a.goTo(p)
	}

	var cmd tea.Cmd
	a.search.input, cmd = a.search.input.Update(k)

	// Every keystroke starts a new debounce window; only the last one fires.
	a.search.seq++
	seq, query := a.search.seq, a.search.input.Value()
	if strings.TrimSpace(query) == "" {
		a.search.results, a.search.busy = nil, false
		return a, cmd
	}
	a.search.busy = true
	return a, tea.Batch(cmd, tea.Tick(debounce, func(time.Time) tea.Msg {
		return debounceMsg{seq: seq, query: query}
	}))
}

func (a *App) favKey(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	n := len(a.cfg.Favourites)
	last := max(0, n-1)
	switch k.String() {
	case "esc", "ctrl+c", "q", "f":
		a.mode = modeChart
	case "up", "k", "ctrl+p":
		a.fav.selected = clamp(a.fav.selected-1, 0, last)
	case "down", "j", "ctrl+n":
		a.fav.selected = clamp(a.fav.selected+1, 0, last)
	case "home", "g":
		a.fav.selected = 0
	case "end", "G":
		a.fav.selected = last
	case "/", "s":
		a.openSearch()
	case "x", "d", "F", "delete", "backspace":
		if n > 0 {
			a.cfg.Toggle(a.cfg.Favourites[a.fav.selected])
			_ = a.cfg.Save(appName)
			a.fav.selected = clamp(a.fav.selected, 0, max(0, len(a.cfg.Favourites)-1))
		}
	case "enter":
		if n == 0 {
			return a, nil
		}
		p := a.cfg.Favourites[a.fav.selected]
		a.mode = modeChart
		return a, a.goTo(p)
	}
	return a, nil
}

// current is the highlighted result, if there is one.
func (s *searchState) current() (geo.Place, bool) {
	if s.selected < 0 || s.selected >= len(s.results) {
		return geo.Place{}, false
	}
	return s.results[s.selected], true
}

// searchUpdate handles search messages in any view, since a reply can land
// after the window is closed.
func (a *App) searchUpdate(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case debounceMsg:
		if msg.seq != a.search.seq {
			return true, nil // superseded by a later keystroke
		}
		return true, lookup(msg.seq, msg.query)
	case searchMsg:
		if msg.seq != a.search.seq {
			return true, nil
		}
		a.search.busy = false
		a.search.results, a.search.err = msg.results, msg.err
		a.search.selected = 0
		return true, nil
	}
	return false, nil
}

func lookup(seq int, query string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		places, err := geo.Search(ctx, appName, query, 8)
		return searchMsg{seq: seq, results: places, err: err}
	}
}

// goTo switches location, remembers it as the default, and loads its forecast.
func (a *App) goTo(p geo.Place) tea.Cmd {
	a.place = p
	a.cfg.Default = &p
	_ = a.cfg.Save(appName) // a read-only config should not break navigation
	return a.reload()
}

func (a *App) toggleFavourite() (tea.Model, tea.Cmd) {
	a.cfg.Toggle(a.place)
	_ = a.cfg.Save(appName)
	return a, nil
}

// searchWindow is the prompt, then whatever the query has turned up. Each
// result carries its own star, so the list doubles as the favourites editor.
func (a *App) searchWindow() window {
	width := a.floatWidth()
	inner := a.floatInner()
	// The input renders itself, so it is told its room rather than clipped.
	a.search.input.SetWidth(max(1, inner-lipgloss.Width(a.search.input.Prompt)-1))

	rows := []wrow{rawRow(a.search.input.View()), textRow(blank())}

	// How the query is getting on, above the results it is filling in.
	switch {
	case a.search.err != nil:
		rows = append(rows, textRow(render.Line{
			{Text: fit(a.search.err.Error(), inner), Colour: render.Red}}))
	case a.search.busy:
		rows = append(rows, note("searching…", inner))
	case len(a.search.results) == 0 && a.search.input.Value() != "":
		rows = append(rows, note("nothing found", inner))
	}

	// The keys live on the tips bar under the header, so the results get the
	// whole of the room left under the prompt.
	room := max(1, a.floatRows()-len(rows))
	lo, hi := scrollTo(len(a.search.results), a.search.selected, room)
	for i := lo; i < hi; i++ {
		rows = append(rows, a.placeRow(a.search.results[i], i == a.search.selected, inner))
	}
	return window{title: "Find a place", width: width, rows: rows}
}

// favWindow is the saved places, in the order they were starred.
func (a *App) favWindow() window {
	width := a.floatWidth()
	inner := a.floatInner()
	if len(a.cfg.Favourites) == 0 {
		return window{title: "Favourites", width: width, rows: []wrow{
			note("nothing saved yet", inner),
			textRow(blank()),
			note("press / to find a place, then tab to star it", inner),
		}}
	}
	lo, hi := scrollTo(len(a.cfg.Favourites), a.fav.selected, a.floatRows())
	rows := make([]wrow, 0, hi-lo)
	for i := lo; i < hi; i++ {
		rows = append(rows, a.placeRow(a.cfg.Favourites[i], i == a.fav.selected, inner))
	}
	return window{title: "Favourites", width: width, rows: rows}
}

// placeRow is one line of either list: cursor, star, name, and coordinates
// down the right edge, each a fixed width so the columns stay columns.
func (a *App) placeRow(p geo.Place, on bool, inner int) wrow {
	marker, colour := "   ", render.Grey
	if on {
		marker, colour = " → ", render.FG
	}
	star, starCol := "  ", render.Grey
	if a.cfg.IsFavourite(p) {
		star, starCol = "★ ", render.Yellow
	}

	line := render.Line{
		{Text: marker, Colour: colour},
		{Text: star, Colour: starCol},
	}
	// The coordinates are dropped before the name is; they only ever settle a
	// tie between two of the same name.
	if nameW := inner - markW - coordW; nameW < 8 {
		line = append(line, render.Span{
			Text: fit(p.Label(), max(0, inner-markW)), Colour: colour})
	} else {
		line = append(line,
			render.Span{Text: fit(p.Label(), nameW), Colour: colour},
			render.Span{
				Text:   fmt.Sprintf("%*s", coordW, fmt.Sprintf("%.2f, %.2f", p.Lat, p.Lon)),
				Colour: render.Grey})
	}
	if !on {
		return textRow(line)
	}
	// Banded as well as arrowed, so the eye need not hunt the marker column.
	return rawRow(highlight(line, inner, render.Dim))
}

// note is a grey aside inside a window.
func note(s string, inner int) wrow {
	return textRow(render.Line{{Text: fit("  "+s, inner), Colour: render.Grey}})
}

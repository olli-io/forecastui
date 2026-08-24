package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/olli-io/forecastui/internal/render"
	"github.com/olli-io/forecastui/internal/theme"
)

// The theme picker recolours the chart as the selection moves, so the palette
// is judged against the forecast it will be read on rather than a swatch.

type themeState struct {
	names    []string
	selected int
	before   *theme.Theme            // put back when the picker is dismissed
	loaded   map[string]*theme.Theme // parsed once each, so moving stays off the disk
	err      error
}

// openThemes stands on the theme already in force, so opening the picker and
// pressing enter changes nothing.
func (a *App) openThemes() {
	t := &a.themes
	t.names = theme.Names(appName)
	t.before, t.err = Active(), nil
	if t.loaded == nil {
		t.loaded = map[string]*theme.Theme{}
	}
	t.loaded[ActiveName()] = Active()

	t.selected = 0
	for i, n := range t.names {
		if n == ActiveName() {
			t.selected = i
			break
		}
	}
	a.mode = modeTheme
}

func (a *App) themeKey(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	last := max(0, len(a.themes.names)-1)
	switch k.String() {
	case "esc", "ctrl+c", "q", "t":
		// Dismissing puts back what was on screen when the picker opened.
		a.wear(a.themes.before)
		a.mode = modeChart
	case "up", "k", "ctrl+p":
		a.themes.selected = clamp(a.themes.selected-1, 0, last)
		a.preview()
	case "down", "j", "ctrl+n":
		a.themes.selected = clamp(a.themes.selected+1, 0, last)
		a.preview()
	case "home", "g":
		a.themes.selected = 0
		a.preview()
	case "end", "G":
		a.themes.selected = last
		a.preview()
	case "enter":
		name, ok := a.themes.current()
		if !ok {
			return a, nil
		}
		a.cfg.Theme = name
		_ = a.cfg.Save(appName) // a read-only config should not undo the choice
		a.mode = modeChart
	}
	return a, nil
}

// current is the highlighted theme name, if there is one.
func (t *themeState) current() (string, bool) {
	if t.selected < 0 || t.selected >= len(t.names) {
		return "", false
	}
	return t.names[t.selected], true
}

// preview wears the highlighted theme. A file that will not parse leaves the
// chart in the colours it had: the picker says so rather than going blank.
func (a *App) preview() {
	name, ok := a.themes.current()
	if !ok {
		return
	}
	t, err := a.loadTheme(name)
	if err != nil {
		a.themes.err = err
		return
	}
	a.themes.err = nil
	a.wear(t)
}

// loadTheme reads a theme once per session. theme.Load seeds the themes
// directory on every call, which is not work to redo on each arrow key.
func (a *App) loadTheme(name string) (*theme.Theme, error) {
	if t, ok := a.themes.loaded[name]; ok {
		return t, nil
	}
	t, err := theme.Load(appName, name)
	if err != nil {
		return nil, err
	}
	a.themes.loaded[name] = t
	return t, nil
}

// wear installs a palette under a running app: Use recolours everything drawn
// from the palette on the next frame, restyle catches what does not.
func (a *App) wear(t *theme.Theme) {
	if t == nil {
		return
	}
	Use(t)
	a.restyle()
}

// restyle re-dresses the widgets that snapshot their colours when they are
// built rather than reading the palette as they draw.
func (a *App) restyle() {
	if a.search.ready {
		a.search.input.SetStyles(inputStyles())
	}
}

// themeWindow is every theme on offer: the files in the themes directory and
// the ones shipped in the binary.
func (a *App) themeWindow() window {
	width := a.floatWidth()
	inner := a.floatInner()
	if len(a.themes.names) == 0 {
		return window{title: "Theme", width: width, rows: []wrow{
			note("no themes found", inner)}}
	}

	var rows []wrow
	if a.themes.err != nil {
		rows = append(rows,
			textRow(render.Line{{Text: fit("  "+shortErr(a.themes.err), inner), Colour: render.Red}}),
			textRow(blank()))
	}
	room := max(1, a.floatRows()-len(rows))
	lo, hi := scrollTo(len(a.themes.names), a.themes.selected, room)
	for i := lo; i < hi; i++ {
		rows = append(rows, a.themeRow(a.themes.names[i], i == a.themes.selected, inner))
	}
	return window{title: "Theme", width: width, rows: rows}
}

// themeRow is one line of the list: cursor, a star on the one written in the
// config, then the name.
func (a *App) themeRow(name string, on bool, inner int) wrow {
	marker, colour := "   ", render.Grey
	if on {
		marker, colour = " → ", render.FG
	}
	star, starCol := "  ", render.Grey
	if name == a.cfg.Theme {
		star, starCol = "★ ", render.Yellow
	}

	line := render.Line{
		{Text: marker, Colour: colour},
		{Text: star, Colour: starCol},
		{Text: fit(name, max(0, inner-markW)), Colour: colour},
	}
	if !on {
		return textRow(line)
	}
	return rawRow(highlight(line, inner, render.Dim))
}

// shortErr drops the file path theme.Load stamps on the front of its errors.
// On a terminal the path is longer than the row and would push the reason off
// the end of it, and the name it names is on the row below anyway.
func shortErr(err error) string {
	if rest, ok := strings.CutPrefix(err.Error(), "theme "); ok {
		if _, after, found := strings.Cut(rest, ": "); found {
			return after
		}
	}
	return err.Error()
}

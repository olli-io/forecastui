package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/olli-io/forecastui/internal/cache"
	"github.com/olli-io/forecastui/internal/fmi"
	"github.com/olli-io/forecastui/internal/geo"
	"github.com/olli-io/forecastui/internal/render"
)

const appName = "forecastui"

// Panel heights, in braille cells. On a short terminal the chart falls back
// through the shorter heights.
const (
	chartCells  = 7
	chartMedium = 5
	chartShort  = 4
	panelCells  = 4
)

// refreshEvery is how often the forecast is re-fetched, and how long one in
// hand counts as current. FMI publishes hourly.
const refreshEvery = cache.Current

// minRefresh is the floor between two requests for the same forecast, so a
// failed load is not retried on every keypress.
const minRefresh = time.Minute

type mode int

const (
	modeChart mode = iota
	modeSearch
	modeFav
	modeTheme
)

// ranges are what tab toggles between.
var ranges = []Span{
	{Hours: 48},
	{Hours: 168, Slots: true},
}

// App is the root model.
type App struct {
	place geo.Place
	span  Span
	cfg   *geo.Config

	client *fmi.Client
	hours  []fmi.Hour
	span0  Span // the range the hours in hand were fetched for
	cols   []render.Column
	scale  render.Scale

	mode    mode
	cursor  int // index into cols
	scroll  int // leftmost visible column
	width   int
	height  int
	nerd    bool // the terminal can draw the Nerd Font glyphs
	loading bool
	stale   bool
	fetched time.Time // when the forecast on screen was fetched from FMI
	lastFet time.Time // when the last request went out, successful or not
	err     error

	search searchState
	fav    favState
	themes themeState
}

// New builds the root model. nerd is settled once at startup; the font cannot
// change under a running program.
func New(p geo.Place, s Span, cfg *geo.Config, nerd bool) tea.Model {
	if cfg == nil {
		cfg = &geo.Config{}
	}
	return &App{
		place: p, span: s, cfg: cfg, nerd: nerd,
		client: fmi.NewClient(),
		width:  80, height: 24,
		loading: true,
	}
}

func (a *App) Init() tea.Cmd {
	// The request waits on the cache read rather than racing it: whether to
	// ask at all is not known until the cache has been looked at.
	return tea.Batch(a.loadCache(), tick())
}

type forecastMsg struct {
	place geo.Place
	span  Span
	hours []fmi.Hour
	err   error
}

type cachedMsg struct {
	place geo.Place
	span  Span
	hours []fmi.Hour
	at    time.Time
}

type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(refreshEvery, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// fetch is the unconditional form; callers go through maybeFetch.
func (a *App) fetch() tea.Cmd {
	place, span := a.place, a.span
	client := a.client
	a.lastFet = time.Now()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		from := time.Now().UTC().Truncate(time.Hour)
		to := from.Add(time.Duration(span.Hours-1) * time.Hour)
		hours, err := client.Fetch(ctx, place.Lat, place.Lon, from, to)
		if err == nil {
			cache.Save(appName, place.Lat, place.Lon, span.Hours, hours)
		}
		return forecastMsg{place: place, span: span, hours: hours, err: err}
	}
}

// current reports whether the forecast on screen is recent enough to leave
// alone. One read back from the cache counts.
func (a *App) current() bool {
	return len(a.hours) > 0 && a.span0.Hours == a.span.Hours && cache.Fresh(a.fetched)
}

// maybeFetch is how every refresh is asked for. A nil command means the
// forecast on screen already stands.
func (a *App) maybeFetch() tea.Cmd {
	if a.current() || time.Since(a.lastFet) < minRefresh {
		return nil
	}
	a.loading = true
	return a.fetch()
}

// loadCache reads the saved forecast. It always answers, empty-handed if need
// be, because the reply decides whether a request goes out at all.
func (a *App) loadCache() tea.Cmd {
	place, span := a.place, a.span
	return func() tea.Msg {
		hours, at, err := cache.Load(appName, place.Lat, place.Lon, span.Hours)
		if err != nil {
			hours, at = nil, time.Time{}
		}
		return cachedMsg{place: place, span: span, hours: hours, at: at}
	}
}

// reload points the view at a new place: the old columns go, and the rate
// limit starts over — it guards repeat requests for one forecast.
func (a *App) reload() tea.Cmd {
	a.hours, a.cols, a.span0 = nil, nil, Span{}
	a.cursor, a.scroll = 0, 0
	a.fetched, a.lastFet = time.Time{}, time.Time{}
	a.loading, a.stale, a.err = true, false, nil
	return a.loadCache()
}

// switchSpan moves to another range. What is drawn stays up while the new
// range loads; span0 marks it as the old range's data.
func (a *App) switchSpan(s Span) tea.Cmd {
	a.span = s
	a.cursor, a.scroll = 0, 0
	a.lastFet = time.Time{}
	a.loading, a.err = true, nil
	return a.loadCache()
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Search replies can land after the overlay has closed.
	if handled, cmd := a.searchUpdate(msg); handled {
		return a, cmd
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		a.clampScroll()
		return a, nil

	case cachedMsg:
		// A later switch, or a live result that got in first, supersedes it.
		if !msg.place.Same(a.place) || msg.span.Hours != a.span.Hours {
			return a, nil
		}
		if len(msg.hours) > 0 && a.span0.Hours != a.span.Hours {
			a.setHours(msg.hours)
			a.fetched = msg.at
			a.stale = !cache.Fresh(msg.at)
		}
		cmd := a.maybeFetch()
		a.loading = cmd != nil
		return a, cmd

	case forecastMsg:
		// A switch made while the request was out supersedes it; the request
		// now in flight for that range clears the flag.
		if !msg.place.Same(a.place) || msg.span.Hours != a.span.Hours {
			return a, nil
		}
		a.loading = false
		if msg.err != nil {
			a.err = msg.err
			return a, nil
		}
		a.err = nil
		a.stale = false
		a.fetched = time.Now()
		a.setHours(msg.hours)
		return a, nil

	case tickMsg:
		// The tick keeps its own time whether or not it fetches.
		return a, tea.Batch(a.maybeFetch(), tick())

	case tea.KeyPressMsg:
		return a.key(msg)
	}
	return a, nil
}

func (a *App) key(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// A floating window owns almost every key while it is up.
	switch a.mode {
	case modeSearch:
		return a.searchKey(k)
	case modeFav:
		return a.favKey(k)
	case modeTheme:
		return a.themeKey(k)
	}
	switch k.String() {
	case "q", "ctrl+c", "esc":
		if a.mode != modeChart {
			a.mode = modeChart
			return a, nil
		}
		return a, tea.Quit

	case "left", "h":
		a.moveCursor(-1)
	case "right", "l":
		a.moveCursor(1)
	case "up", "k", "L":
		a.moveCursor(a.dayStep())
	case "down", "j", "H":
		a.moveCursor(-a.dayStep())
	case "pgup":
		a.moveCursor(-a.visible())
	case "pgdown":
		a.moveCursor(a.visible())
	case "home", "g":
		a.cursor = 0
		a.clampScroll()
	case "end", "G":
		a.cursor = len(a.cols) - 1
		a.clampScroll()

	case "tab":
		return a, a.switchSpan(nextRange(a.span))
	case "shift+tab":
		return a, a.switchSpan(prevRange(a.span))

	case "r":
		return a, a.maybeFetch()

	case "/", "s":
		a.openSearch()
		return a, nil

	case "f":
		a.openFavourites()
		return a, nil
	case "F":
		return a.toggleFavourite()

	case "t":
		a.openThemes()
		return a, nil
	}
	return a, nil
}

func nextRange(s Span) Span {
	for i, r := range ranges {
		if r.Hours == s.Hours {
			return ranges[(i+1)%len(ranges)]
		}
	}
	return ranges[0]
}

func prevRange(s Span) Span {
	for i, r := range ranges {
		if r.Hours == s.Hours {
			return ranges[(i-1+len(ranges))%len(ranges)]
		}
	}
	return ranges[len(ranges)-1]
}

func (a *App) setHours(hours []fmi.Hour) {
	a.hours, a.span0 = hours, a.span
	a.cols = render.Columns(hours, a.span.Slots, a.place.Lat, a.place.Lon)
	a.scale = render.NewScale(a.cols)
	if a.cursor >= len(a.cols) {
		a.cursor = max(0, len(a.cols)-1)
	}
	a.clampScroll()
}

func (a *App) moveCursor(by int) {
	if len(a.cols) == 0 {
		return
	}
	a.cursor = clamp(a.cursor+by, 0, len(a.cols)-1)
	a.clampScroll()
}

// clampScroll keeps the cursor inside the viewport, scrolling sideways rather
// than paging.
func (a *App) clampScroll() {
	n := a.visible()
	if len(a.cols) <= n {
		a.scroll = 0
		return
	}
	if a.cursor < a.scroll {
		a.scroll = a.cursor
	}
	if a.cursor >= a.scroll+n {
		a.scroll = a.cursor - n + 1
	}
	a.scroll = clamp(a.scroll, 0, len(a.cols)-n)
}

func (a *App) visible() int { return render.Fits(a.width) }

// drawn is how many columns the chart puts on screen; the boxes measure
// themselves against it.
func (a *App) drawn() int { return max(0, min(a.visible(), len(a.cols)-a.scroll)) }

// opts is what every panel of the chart view is drawn from.
func (a *App) opts() render.Opts {
	return render.Opts{
		Start: a.scroll, Count: a.visible(),
		Slots: a.span.Slots, Nerd: a.nerd, Cursor: a.cursor,
	}
}

// dayStep is how many columns make up a day, read off the columns themselves
// so it follows whatever step the current view is drawn in.
func (a *App) dayStep() int {
	if len(a.cols) < 2 {
		return 1
	}
	step := a.cols[1].At.Sub(a.cols[0].At)
	if step <= 0 {
		return 1
	}
	return max(1, int(24*time.Hour/step))
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (a *App) View() tea.View {
	v := tea.NewView(a.render())
	v.AltScreen = true
	return v
}

func (a *App) render() string {
	switch a.mode {
	case modeSearch:
		return a.overlay(a.searchWindow())
	case modeFav:
		return a.overlay(a.favWindow())
	case modeTheme:
		return a.overlay(a.themeWindow())
	}
	return a.chartView()
}

func (a *App) chartView() string {
	if len(a.cols) == 0 {
		return a.statusOnly()
	}

	opts := a.opts()

	// Temperature, rain and wind are boxes stacked on one time line, each
	// closing with its own rule, so every panel is measured before any is
	// appended.
	rule := render.Rule(a.cols, opts, fmt.Sprintf("%.1f°C", a.scale.Lo))
	zeroRule := render.Rule(a.cols, opts, "0")
	hours := render.HourLabels(a.cols, opts)
	temps := render.TempLabels(a.cols, opts)
	rains := render.RainLabels(a.cols, a.scale, opts)
	frame := 0
	if a.cursor >= a.scroll && a.cursor < a.scroll+a.visible() {
		frame = 2 // the cursor frame's own cap and foot
	}
	// What stands under the chart whatever else is dropped.
	fixed := 1 + len(hours) + 1 + frame
	if rains != nil {
		fixed++
	}

	// The header box costs two rows more than a plain line. Below three cells
	// a chart stops being one, so the frame gives way before the bars do.
	hdr := a.header()
	if a.height-len(hdr)-1-fixed < 3 {
		hdr = []render.Line{a.headerLine()}
	}
	lines := append([]render.Line{}, hdr...)
	lines = append(lines, a.keyLine())
	budget := a.height - len(lines)

	// Panels are dropped from the bottom up on a short terminal, so the
	// temperature chart always survives.
	chartH := chartCells
	if budget < 20 {
		chartH = chartMedium
	}
	if budget < 14 {
		chartH = chartShort
	}
	// The rain figures give way before the chart does; the bars still read.
	if budget-fixed < chartH && rains != nil {
		rains, fixed = nil, fixed-1
	}
	chartH = max(1, min(chartH, budget-fixed))
	chart := render.Chart(a.cols, a.scale, withHeight(opts, chartH))
	budget -= len(chart) + fixed

	sky := render.Sky(a.cols, opts)
	if budget >= 3 && sky != nil {
		budget -= len(sky)
	} else {
		sky = nil
	}
	// The detail pane outranks the wind panel: it sits under the cursor's
	// foot, with no gap, and the arrow points at it.
	detail := a.detail()
	if budget >= len(detail) {
		budget -= len(detail)
	} else {
		detail = nil
	}
	// Rain gets the space before wind does. Its panel costs its bars plus the
	// rule under them.
	rain := render.Rain(a.cols, a.scale, withHeight(opts, panelCells))
	if rain == nil || budget < len(rain)+1 {
		rain = nil
	} else {
		budget -= len(rain) + 1
	}
	// A wind panel costs its bars plus its rule, direction row and speeds.
	wind := render.Wind(a.cols, a.scale, withHeight(opts, panelCells))
	if budget < len(wind)+3 || wind == nil {
		wind = nil
	}

	// Everything the cursor frame runs through, as one block so the frame can
	// be drawn down it in one pass.
	stack := append([]render.Line{}, chart...)
	stack = append(stack, rule)
	if rain != nil {
		stack = append(stack, rain...)
		stack = append(stack, zeroRule)
	}
	if wind != nil {
		stack = append(stack, wind...)
		stack = append(stack, zeroRule)
	}
	stack = append(stack, hours...)
	stack = append(stack, sky...)
	stack = append(stack, temps)
	if rains != nil {
		stack = append(stack, rains)
	}
	if wind != nil {
		stack = append(stack, render.WindDirs(a.cols, opts))
		stack = append(stack, render.WindSpeeds(a.cols, opts))
	}
	lines = append(lines, render.Cursor(stack, a.cols, opts)...)
	lines = append(lines, detail...)

	return Paint(lines, true, a.width)
}

func withHeight(o render.Opts, h int) render.Opts {
	o.CellH = h
	return o
}

// pad pushes the footer to the bottom of the screen. used counts body lines;
// one more for the newline already written.
func (a *App) pad(used int) string {
	if n := a.height - used - 1; n > 0 {
		return strings.Repeat("\n", n)
	}
	return ""
}

func (a *App) statusOnly() string {
	msg := "loading forecast…"
	if a.err != nil {
		msg = "could not load the forecast: " + a.err.Error()
	} else if !a.loading {
		msg = "no forecast data for " + a.place.Label()
	}
	lines := []render.Line{
		blank(),
		{{Text: "  " + a.place.Label(), Colour: render.FG}},
		blank(),
		{{Text: "  " + msg, Colour: render.Grey}},
	}
	return Paint(lines, true, a.width) + "\n" + a.pad(len(lines)) + a.footer()
}

// header is the place and span, boxed, with the ranges set into the top edge.
func (a *App) header() []render.Line {
	return boxedTop(rangeTabs(a.span), a.headerSpans(), a.drawn(), a.width, render.Grey)
}

// headerLine is the same reading as a plain indented row, for a terminal too
// short for the frame.
func (a *App) headerLine() render.Line {
	indent := strings.Repeat(" ", keyIndent)
	return append(render.Line{{Text: indent, Colour: render.Grey}}, a.headerSpans()...)
}

func (a *App) headerSpans() render.Line {
	line := headerText(a.place.Label(), a.cols, a.span.Slots)
	var notes []string
	if a.cfg.IsFavourite(a.place) {
		notes = append(notes, "★")
	}
	if a.loading {
		notes = append(notes, "refreshing…")
	}
	if a.stale {
		notes = append(notes, "cached "+ago(a.fetched))
	} else if !a.fetched.IsZero() {
		notes = append(notes, "updated "+ago(a.fetched))
	}
	if a.err != nil {
		notes = append(notes, "offline")
	}
	if len(notes) > 0 {
		line = append(line, render.Span{
			Text:   "  " + strings.Join(notes, " · "),
			Colour: render.Grey,
		})
	}
	return line
}

// rangeTabs sets the ranges into the header box's top edge, the one on screen
// lit and the other lying along the border beside it.
func rangeTabs(active Span) render.Line {
	line := render.Line{}
	for _, r := range ranges {
		col := render.FG
		if r == active {
			col = render.Yellow
		}
		line = append(line,
			render.Span{Text: "─", Colour: render.Grey},
			render.Span{Text: r.Label(), Colour: col})
	}
	return append(line, render.Span{Text: "─", Colour: render.Grey})
}

func ago(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	d := time.Since(t).Round(time.Minute)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh ago", int(d.Hours()))
}

// hint is one entry of a shortcut list. The key and its description are held
// apart so the key can be lit and the words grey. No key means an instruction.
type hint struct{ key, what string }

// chartHints is the widest shortcut list that still fits.
func (a *App) chartHints() []hint {
	return a.fitHints(
		[]hint{{"←→", "hour"}, {"↑↓", "day"}, {"tab", "range"}, {"/", "place"},
			{"f", "favs"}, {"t", "theme"}, {"r", "refresh"}, {"q", "quit"}},
		[]hint{{"←→", "hour"}, {"↑↓", "day"}, {"tab", "range"}, {"/", "place"},
			{"f", "favs"}, {"r", "refresh"}, {"q", "quit"}},
		[]hint{{"←→", "hour"}, {"tab", "range"}, {"/", "place"}, {"f", "favs"},
			{"q", "quit"}},
		[]hint{{"←→", ""}, {"tab", ""}, {"/", ""}, {"f", ""}, {"q", "quit"}})
}

// fitHints picks the widest list that still fits beside the footer's indent,
// so a list is shortened rather than clipped.
func (a *App) fitHints(lists ...[]hint) []hint {
	for _, hs := range lists {
		if keyIndent+lipgloss.Width(hintLine(hs).Plain()) <= a.width {
			return hs
		}
	}
	return []hint{{"q", "quit"}}
}

// footerHints is the shortcut list under the view. A floating window has its
// own, since the chart's keys are typed into its prompt while it is up.
func (a *App) footerHints() []hint {
	switch a.mode {
	case modeChart:
		return a.chartHints()
	case modeSearch:
		return a.fitHints(
			[]hint{{"", "type to search"}, {"↑↓", "pick"}, {"↵", "select"},
				{"tab", "favourite"}, {"esc", "close"}},
			[]hint{{"", "type to search"}, {"↑↓", "pick"}, {"↵", "select"},
				{"tab", "star"}, {"esc", "close"}},
			[]hint{{"↑↓", "pick"}, {"↵", "select"}, {"tab", "star"}, {"esc", "close"}})
	case modeFav:
		return a.fitHints(
			[]hint{{"↑↓", "pick"}, {"↵", "go"}, {"x", "unstar"}, {"/", "search"},
				{"esc", "close"}},
			[]hint{{"↑↓", ""}, {"↵", "go"}, {"x", "unstar"}, {"esc", "close"}},
			[]hint{{"↵", "go"}, {"x", "unstar"}, {"esc", ""}})
	case modeTheme:
		return a.fitHints(
			[]hint{{"↑↓", "preview"}, {"↵", "keep"}, {"esc", "cancel"}},
			[]hint{{"↑↓", ""}, {"↵", "keep"}, {"esc", "cancel"}},
			[]hint{{"↵", "keep"}, {"esc", ""}})
	}
	return []hint{{"esc", "back"}, {"q", "quit"}}
}

// hintLine writes a shortcut list out: every key lit, its words in the shade.
func hintLine(hs []hint) render.Line {
	var line render.Line
	for i, h := range hs {
		if i > 0 {
			line = append(line, render.Span{Text: " · ", Colour: render.Grey})
		}
		if h.key != "" {
			line = append(line, render.Span{Text: h.key, Colour: render.Yellow})
		}
		if h.what == "" {
			continue
		}
		gap := ""
		if h.key != "" {
			gap = " "
		}
		line = append(line, render.Span{Text: gap + h.what, Colour: render.Grey})
	}
	return line
}

// keyLine is the shortcut list as a row of the body, hung under the header box
// rather than at the foot of the screen.
func (a *App) keyLine() render.Line {
	keys := hintLine(a.footerHints())
	// Indented like a caption under the box above; a terminal too narrow to
	// spare the indent gives it up.
	indent := min(keyIndent, max(0, a.width-lipgloss.Width(keys.Plain())))
	return append(render.Line{{Text: strings.Repeat(" ", indent), Colour: render.Grey}},
		keys...)
}

// footer is the same list pinned to the bottom, for views with no header box.
func (a *App) footer() string {
	return Paint([]render.Line{a.keyLine()}, true, a.width)
}

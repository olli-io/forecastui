package ui

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/charmbracelet/x/term"

	"github.com/olli-io/forecastui/internal/fmi"
	"github.com/olli-io/forecastui/internal/geo"
	"github.com/olli-io/forecastui/internal/render"
)

// Span is how much forecast to show and how densely.
type Span struct {
	Hours int
	Slots bool // aggregate to 3 h steps
}

// Label names a range the way the header tabs show it. The two ranges the app
// toggles between are named rather than measured: the header line beside the
// tab already spells the span out to the hour, so a second reading of the same
// thing in the border says nothing. Any other length the flags ask for is
// still shown as the number of hours it is.
func (s Span) Label() string {
	switch {
	case s.Hours >= 168:
		return "week"
	case s.Hours == 48:
		return "day"
	}
	return fmt.Sprintf("%dh", s.Hours)
}

// Once renders a single static chart, the shell script's whole job. It draws
// the same stack the interactive view does — boxed header, chart, rain and
// wind panels, the sky row and the readings under it, and the cursor frame
// with its detail box, standing on the current hour — bar the shortcut list,
// which names keys a dump has nothing to do with. A dump is not bounded by
// the terminal's height, so no panel is dropped to fit.
//
// Width of zero means "measure the terminal"; nerd says whether the terminal
// can draw the Nerd Font glyphs, settled by the caller as it is for the app.
func Once(place geo.Place, hours []fmi.Hour, span Span, width int, nerd bool) (string, error) {
	if len(hours) == 0 {
		return "", errors.New("no forecast hours to draw")
	}
	if width <= 0 {
		width = TerminalWidth()
	}

	cols := render.Columns(hours, span.Slots, place.Lat, place.Lon)
	if len(cols) == 0 {
		return "", errors.New("no forecast hours fell on a 3 h step — try a longer range")
	}
	sc := render.NewScale(cols)

	// A static dump keeps the near future, which is what the reader came for.
	n := render.Fits(width)
	// Nothing moves the cursor here, so it stands where the reader is: on the
	// hour the command was run in, with the detail box under it reading out
	// the weather right now.
	o := render.Opts{
		Start: 0, Count: n,
		Slots: span.Slots, Nerd: nerd, Cursor: nowColumn(cols, time.Now()),
	}

	// The header box, then a blank row where the interactive view hangs its
	// shortcut list: either way the box's bottom edge is clear of the chart.
	drawn := min(n, len(cols))
	lines := boxedTop(activeTab(span), headerText(place.Label(), cols, span.Slots),
		drawn, width, render.Grey)
	lines = append(lines, blank())

	// Temperature, rain and wind are boxes stacked on one time line: each
	// closes with its own rule, and only the lowest carries the hour labels.
	// Everything the cursor frame runs through is built as one block, so the
	// frame can be drawn down it in one pass.
	zero := render.Rule(cols, o, "0")
	stack := append([]render.Line{}, render.Chart(cols, sc, withHeight(o, chartCells))...)
	stack = append(stack, render.Rule(cols, o, fmt.Sprintf("%.1f°C", sc.Lo)))
	// The rain panel is skipped outright on a dry forecast; the legend below
	// still reports that there is none.
	if rain := render.Rain(cols, sc, withHeight(o, panelCells)); rain != nil {
		stack = append(stack, rain...)
		stack = append(stack, zero)
	}
	wind := render.Wind(cols, sc, withHeight(o, panelCells))
	if wind != nil {
		stack = append(stack, wind...)
		stack = append(stack, zero)
	}
	// Hours, then the readings that hang off them: the sky glyph, the
	// temperature under it, and the wind direction when that panel is up.
	stack = append(stack, render.HourLabels(cols, o)...)
	stack = append(stack, render.Sky(cols, o)...)
	stack = append(stack, render.TempLabels(cols, o))
	if rains := render.RainLabels(cols, sc, o); rains != nil {
		stack = append(stack, rains)
	}
	if wind != nil {
		stack = append(stack, render.WindDirs(cols, o), render.WindSpeeds(cols, o))
	}
	lines = append(lines, render.Cursor(stack, cols, o)...)
	lines = append(lines, detail(cols, o, drawn, width, nerd)...)

	// Every row is measured off Fits already, so the clip is a guarantee
	// rather than a change: no line can wrap and shear the braille grid
	// across two terminal rows.
	return Paint(lines, ColourEnabled(), width), nil
}

// nowColumn is the column the reader is standing in: the last one whose hour
// has already begun, or the first when the whole forecast is still ahead.
func nowColumn(cols []render.Column, now time.Time) int {
	at := 0
	for i, c := range cols {
		if c.At.After(now) {
			break
		}
		at = i
	}
	return at
}

// activeTab is the span set into the header box's top edge, drawn the way
// rangeTabs draws the lit one. The range it is not showing is left off: a
// static dump has no tab key to reach the other one with.
func activeTab(s Span) render.Line {
	return render.Line{
		{Text: "─", Colour: render.Grey},
		{Text: s.Label(), Colour: render.Yellow},
		{Text: "─", Colour: render.Grey},
	}
}

// TerminalWidth reports the usable width, falling back to the classic 80.
func TerminalWidth() int {
	if w, _, err := term.GetSize(os.Stdout.Fd()); err == nil && w > 0 {
		return w
	}
	if n, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && n > 0 {
		return n
	}
	return 80
}

// ColourEnabled follows the same rules the script did: colour only on a TTY,
// and never when NO_COLOR is set.
func ColourEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	return term.IsTerminal(os.Stdout.Fd())
}

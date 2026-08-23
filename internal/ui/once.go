package ui

import (
	"errors"
	"fmt"
	"os"
	"strconv"

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

// Label names a range the way the header tabs show it: hours while they are
// still a number worth reading, days once a week or more is in view.
func (s Span) Label() string {
	if s.Hours >= 168 {
		return fmt.Sprintf("%dd", s.Hours/24)
	}
	return fmt.Sprintf("%dh", s.Hours)
}

// Once renders a single static chart, the shell script's whole job. Width of
// zero means "measure the terminal".
func Once(place geo.Place, hours []fmi.Hour, span Span, width int) (string, error) {
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

	lines := []render.Line{Header(place.Label(), cols, span.Slots), blank()}
	o := render.Opts{Start: 0, Count: n, Slots: span.Slots, Cursor: -1}
	lines = append(lines, render.Chart(cols, sc, o)...)
	// The rain panel is skipped outright on a dry forecast; the legend below
	// still reports that there is none.
	if rain := render.Rain(cols, sc, withHeight(o, panelCells)); rain != nil {
		lines = append(lines, render.Rule(cols, o, fmt.Sprintf("%.1f°C", sc.Lo)))
		lines = append(lines, rain...)
		lines = append(lines, render.Rule(cols, o, "0"))
		lines = append(lines, render.HourLabels(cols, o)...)
		lines = append(lines, render.TempLabels(cols, o))
	} else {
		lines = append(lines, render.Axis(cols, sc, o)...)
	}
	if rains := render.RainLabels(cols, sc, o); rains != nil {
		lines = append(lines, rains)
	}
	lines = append(lines, blank(), Legend(cols, sc, span.Slots))
	if note := Note(span.Slots); note != nil {
		lines = append(lines, note)
	}
	if n < len(cols) {
		lines = append(lines, render.Line{{
			Text:   truncNote(len(cols)-n, n, span.Slots),
			Colour: render.Grey,
		}})
	}
	return Paint(lines, ColourEnabled(), 0), nil
}

func truncNote(dropped, shown int, slots bool) string {
	unit := "hours"
	if slots {
		unit = "steps"
	}
	// At full width the terminal is no longer what is holding the chart back,
	// so telling the reader to widen it would only waste their time.
	fix := "widen the terminal, or drop -once to scroll"
	if shown >= render.MaxCols {
		fix = "drop -once to scroll"
	}
	return fmt.Sprintf("  %d more %s beyond the right edge — %s", dropped, unit, fix)
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

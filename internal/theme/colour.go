package theme

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// names are the sixteen terminal colours, so a theme can defer to whatever the
// terminal itself is set to.
var names = map[string]ansi.BasicColor{
	"black":   lipgloss.Black,
	"red":     lipgloss.Red,
	"green":   lipgloss.Green,
	"yellow":  lipgloss.Yellow,
	"blue":    lipgloss.Blue,
	"magenta": lipgloss.Magenta,
	"cyan":    lipgloss.Cyan,
	"white":   lipgloss.White,

	"bright-black":   lipgloss.BrightBlack,
	"bright-red":     lipgloss.BrightRed,
	"bright-green":   lipgloss.BrightGreen,
	"bright-yellow":  lipgloss.BrightYellow,
	"bright-blue":    lipgloss.BrightBlue,
	"bright-magenta": lipgloss.BrightMagenta,
	"bright-cyan":    lipgloss.BrightCyan,
	"bright-white":   lipgloss.BrightWhite,
}

// ParseColour reads one theme value: a hex string, a 0-255 palette index, one
// of the sixteen colour names, or "default" for the terminal's own foreground.
// lipgloss.Color answers NoColor rather than an error on malformed input, so
// the shape is checked here first.
func ParseColour(s string) (color.Color, error) {
	v := strings.ToLower(strings.TrimSpace(s))
	switch {
	case v == "":
		return nil, fmt.Errorf("empty colour")

	case v == "default", v == "none":
		return lipgloss.NoColor{}, nil

	case strings.HasPrefix(v, "#"):
		if len(v) != 4 && len(v) != 7 {
			return nil, fmt.Errorf("bad hex colour %q, want #rgb or #rrggbb", s)
		}
		if _, err := strconv.ParseUint(v[1:], 16, 32); err != nil {
			return nil, fmt.Errorf("bad hex colour %q", s)
		}
		return lipgloss.Color(v), nil
	}

	// "bright-black" and "brightblack" both read naturally, so accept either.
	if c, ok := names[v]; ok {
		return c, nil
	}
	if c, ok := names[strings.Replace(v, "bright", "bright-", 1)]; ok {
		return c, nil
	}

	if n, err := strconv.Atoi(v); err == nil {
		if n < 0 || n > 255 {
			return nil, fmt.Errorf("colour index %d out of range, want 0-255", n)
		}
		return lipgloss.Color(v), nil
	}

	return nil, fmt.Errorf("unknown colour %q, want a hex value, a 0-255 index, "+
		"a colour name such as \"bright-blue\", or \"default\"", s)
}

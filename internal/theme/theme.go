// Package theme loads the colour palette from disk. A theme is a TOML file
// naming the ten slots internal/render paints with; internal/ui maps them onto
// lipgloss.
package theme

import (
	"image/color"
	"sort"

	"github.com/olli-io/forecastui/internal/render"
)

// Theme is a full palette: every slot is populated, missing ones having been
// filled in from the default.
type Theme struct {
	Name    string
	Colours map[render.Colour]color.Color
}

// slots are the legal keys in a theme file. Anything else is a typo.
var slots = map[string]render.Colour{
	"grey":   render.Grey,
	"fg":     render.FG,
	"purple": render.Purple,
	"aqua":   render.Aqua,
	"green":  render.Green,
	"yellow": render.Yellow,
	"orange": render.Orange,
	"red":    render.Red,
	"blue":   render.Blue,
	"dim":    render.Dim,
}

// slotNames lists the legal keys in file order, for error messages.
func slotNames() []string {
	out := make([]string, 0, len(slots))
	for k := range slots {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// clone copies a theme so an overlay cannot write through to its base.
func (t *Theme) clone(name string) *Theme {
	out := &Theme{Name: name, Colours: make(map[render.Colour]color.Color, len(t.Colours))}
	for k, v := range t.Colours {
		out.Colours[k] = v
	}
	return out
}

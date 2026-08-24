package ui

import (
	"testing"

	"github.com/olli-io/forecastui/internal/render"
	"github.com/olli-io/forecastui/internal/theme"
)

// Use must reach both the styles the chart is painted with and the raw colours
// the highlight band and the cursor are drawn from.
func TestUseInstallsThePalette(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Cleanup(func() { Use(theme.Fallback()) })

	g, err := theme.Load("forecastui", "gruvbox-material")
	if err != nil {
		t.Fatal(err)
	}
	Use(g)

	for slot := range g.Colours {
		if got := Colour(slot); got != g.Colours[slot] {
			t.Errorf("slot %d is %v, want %v", slot, got, g.Colours[slot])
		}
		if got := Style(slot).GetForeground(); got != g.Colours[slot] {
			t.Errorf("slot %d styles as %v, want %v", slot, got, g.Colours[slot])
		}
	}
}

// Left alone, bubbles paints the search prompt in its own colours.
func TestSearchInputFollowsTheTheme(t *testing.T) {
	st := inputStyles()
	for _, tc := range []struct {
		what string
		got  any
		want any
	}{
		{"prompt", st.Focused.Prompt.GetForeground(), Colour(render.Yellow)},
		{"text", st.Focused.Text.GetForeground(), Colour(render.FG)},
		{"placeholder", st.Focused.Placeholder.GetForeground(), Colour(render.Dim)},
		{"cursor", st.Cursor.Color, Colour(render.FG)},
	} {
		if tc.got != tc.want {
			t.Errorf("%s is %v, want %v", tc.what, tc.got, tc.want)
		}
	}
}

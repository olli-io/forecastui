package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/olli-io/forecastui/internal/geo"
	"github.com/olli-io/forecastui/internal/render"
	"github.com/olli-io/forecastui/internal/theme"
)

// themeApp is an app whose config directory is a temporary one, so a picked
// theme is never written to the config the developer is running under. The
// palette is a package global, so it is put back afterwards.
func themeApp(t *testing.T, w, h int) *App {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	before := Active()
	t.Cleanup(func() { Use(before) })
	return newTestApp(t, w, h, 48)
}

func TestThemeWindowFloatsOverTheChart(t *testing.T) {
	a := themeApp(t, 100, 30)
	press(a, "t")
	out := stripANSI(a.render())
	for _, want := range []string{"Theme", "everforest", "gruvbox-material", "day"} {
		if !strings.Contains(out, want) {
			t.Errorf("the overlaid view is missing %q:\n%s", want, out)
		}
	}
}

// The picker opens on the theme the chart is already wearing, so opening it and
// pressing enter changes nothing.
func TestThemePickerOpensOnTheActiveTheme(t *testing.T) {
	a := themeApp(t, 100, 30)
	press(a, "t")
	if got, _ := a.themes.current(); got != ActiveName() {
		t.Errorf("opened on %q, want the active %q", got, ActiveName())
	}
}

// Moving the selection recolours the chart there and then; esc puts it back.
func TestThemePreviewAppliesAndEscRestores(t *testing.T) {
	a := themeApp(t, 100, 30)
	was, wasBlue := ActiveName(), Colour(render.Blue)

	press(a, "t")
	press(a, "down")

	name, _ := a.themes.current()
	if ActiveName() != name {
		t.Fatalf("moving selected %q but the palette is %q", name, ActiveName())
	}
	if ActiveName() == was {
		t.Fatal("moving down did not change the theme")
	}
	want, err := theme.Load(appName, name)
	if err != nil {
		t.Fatal(err)
	}
	if got := Colour(render.Blue); got != want.Colours[render.Blue] {
		t.Errorf("blue is %v, want %v", got, want.Colours[render.Blue])
	}

	press(a, "esc")
	if ActiveName() != was {
		t.Errorf("esc left the theme on %q, want %q", ActiveName(), was)
	}
	if got := Colour(render.Blue); got != wasBlue {
		t.Errorf("esc left blue as %v, want %v", got, wasBlue)
	}
	if a.mode != modeChart {
		t.Error("esc did not close the picker")
	}
}

// Enter keeps the theme and writes it where the next launch will find it.
func TestThemeEnterSavesToConfig(t *testing.T) {
	a := themeApp(t, 100, 30)
	press(a, "t")
	press(a, "down")
	name, _ := a.themes.current()
	press(a, "enter")

	if a.mode != modeChart {
		t.Error("enter did not close the picker")
	}
	if ActiveName() != name {
		t.Errorf("enter left the theme on %q, want %q", ActiveName(), name)
	}
	cfg, err := geo.Load(appName)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Theme != name {
		t.Errorf("config saved theme %q, want %q", cfg.Theme, name)
	}
}

// A theme file that will not parse must not blank the chart: the colours stay
// as they were and the picker says what is wrong.
func TestBrokenThemeKeepsTheColoursAndReports(t *testing.T) {
	a := themeApp(t, 100, 30)
	dir, err := theme.Dir(appName)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// "aaa" sorts ahead of every shipped theme, so it is one step up from any
	// of them.
	if err := os.WriteFile(filepath.Join(dir, "aaa-broken.toml"),
		[]byte("nope = \"#ffffff\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	press(a, "t")
	was := Colour(render.Blue)
	press(a, "home") // straight onto the broken file, so nothing good is worn in between
	if name, _ := a.themes.current(); name != "aaa-broken" {
		t.Fatalf("the top of the list is %q, want aaa-broken", name)
	}
	if a.themes.err == nil {
		t.Error("a broken theme was previewed without an error")
	}
	if got := Colour(render.Blue); got != was {
		t.Errorf("a broken theme changed blue to %v, want %v left alone", got, was)
	}
	if out := stripANSI(a.render()); !strings.Contains(out, "unknown key") {
		t.Errorf("the picker does not report the broken file:\n%s", out)
	}
}

// The search prompt snapshots its colours when it is built, so a theme changed
// under a running app has to be pushed into it.
func TestThemeChangeReachesTheSearchPrompt(t *testing.T) {
	a := themeApp(t, 100, 30)
	a.openSearch() // builds the input, snapshotting the current palette
	a.mode = modeChart

	press(a, "t")
	press(a, "down")
	press(a, "enter")

	if got := a.search.input.Styles().Focused.Prompt.GetForeground(); got != Colour(render.Yellow) {
		t.Errorf("the prompt is still %v, want the new theme's %v", got, Colour(render.Yellow))
	}
}

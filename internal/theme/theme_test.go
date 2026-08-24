package theme

import (
	"image/color"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/olli-io/forecastui/internal/render"
)

// tempConfig points the config directory at a fresh temporary one.
func tempConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	return filepath.Join(dir, "testapp", "themes")
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestShippedThemesAreComplete(t *testing.T) {
	tempConfig(t)
	for _, name := range embedded() {
		got, err := Load("testapp", name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for key, slot := range slots {
			if _, ok := got.Colours[slot]; !ok {
				t.Errorf("%s: no colour for %q", name, key)
			}
		}
	}
}

// Nothing chosen anywhere lands on everforest, and every shipped theme is
// written out beside it to be read and copied.
func TestNothingChosenLandsOnTheFallback(t *testing.T) {
	dir := tempConfig(t)
	got, err := Load("testapp", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != FallbackName {
		t.Errorf("no theme chosen gives %q, want %q", got.Name, FallbackName)
	}

	shipped := embedded()
	if len(shipped) < 3 {
		t.Errorf("only %d themes ship: %v", len(shipped), shipped)
	}
	for _, name := range shipped {
		if _, err := os.Stat(filepath.Join(dir, name+".toml")); err != nil {
			t.Errorf("%s was not shipped into the themes directory: %v", name, err)
		}
	}
	if !slices.Contains(shipped, FallbackName) {
		t.Errorf("the fallback %q is not among the shipped themes %v", FallbackName, shipped)
	}
}

func TestGruvboxKeepsTheOriginalPalette(t *testing.T) {
	tempConfig(t)
	got, err := Load("testapp", "gruvbox-material")
	if err != nil {
		t.Fatal(err)
	}
	if c := got.Colours[render.FG]; c != (color.RGBA{R: 0xd5, G: 0xc4, B: 0xa1, A: 0xff}) {
		t.Errorf("fg is %v, want #d5c4a1", c)
	}
}

func TestParseColour(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want color.Color
	}{
		{"#d5c4a1", color.RGBA{R: 0xd5, G: 0xc4, B: 0xa1, A: 0xff}},
		{"#fff", color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}},
		{"0", ansi.BasicColor(0)},
		{"15", ansi.BasicColor(15)},
		{"16", lipgloss.ANSIColor(16)},
		{"255", lipgloss.ANSIColor(255)},
		{"blue", ansi.BasicColor(4)},
		{"bright-blue", ansi.BasicColor(12)},
		{"brightblue", ansi.BasicColor(12)},
		{"  Bright-Blue  ", ansi.BasicColor(12)},
		{"default", lipgloss.NoColor{}},
		{"none", lipgloss.NoColor{}},
	} {
		got, err := ParseColour(tc.in)
		if err != nil {
			t.Errorf("%q: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%q is %v, want %v", tc.in, got, tc.want)
		}
	}

	for _, bad := range []string{"", "  ", "#12", "#12345", "#gggggg", "256", "-1", "puce"} {
		if got, err := ParseColour(bad); err == nil {
			t.Errorf("%q should not parse, got %v", bad, got)
		}
	}
}

func TestPartialThemeInheritsTheRest(t *testing.T) {
	dir := tempConfig(t)
	write(t, filepath.Join(dir, "mine.toml"), "blue = \"#ff00ff\"\n")

	got, err := Load("testapp", "mine")
	if err != nil {
		t.Fatal(err)
	}
	if c := got.Colours[render.Blue]; c != (color.RGBA{R: 0xff, B: 0xff, A: 0xff}) {
		t.Errorf("blue is %v, want #ff00ff", c)
	}
	if got.Colours[render.Red] != Default().Colours[render.Red] {
		t.Error("red should have been inherited from the default")
	}
	if Default().Colours[render.Blue] == got.Colours[render.Blue] {
		t.Error("the overlay wrote through to the default")
	}
}

func TestDroppedInFileShadowsTheShippedOne(t *testing.T) {
	dir := tempConfig(t)
	write(t, filepath.Join(dir, "gruvbox-material.toml"), "fg = \"#000000\"\n")

	got, err := Load("testapp", "gruvbox-material")
	if err != nil {
		t.Fatal(err)
	}
	if c := got.Colours[render.FG]; c != (color.RGBA{A: 0xff}) {
		t.Errorf("fg is %v, want the dropped-in #000000", c)
	}
}

func TestLoadByPath(t *testing.T) {
	tempConfig(t)
	path := filepath.Join(t.TempDir(), "elsewhere.toml")
	write(t, path, "green = \"#00ff00\"\n")

	got, err := Load("testapp", path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "elsewhere" {
		t.Errorf("name is %q, want elsewhere", got.Name)
	}
	if _, err := Load("testapp", filepath.Join(t.TempDir(), "gone.toml")); err == nil {
		t.Error("a missing file should error")
	}
}

func TestSeedWritesTheShippedThemes(t *testing.T) {
	dir := tempConfig(t)
	if _, err := Load("testapp", ""); err != nil {
		t.Fatal(err)
	}
	for _, name := range embedded() {
		if _, err := os.Stat(filepath.Join(dir, name+".toml")); err != nil {
			t.Errorf("%s was not written out: %v", name, err)
		}
	}

	// An edited theme is left as it stands; a missing one is written back, so
	// an upgrade hands over the themes it added.
	edited := filepath.Join(dir, "gruvbox-material.toml")
	write(t, edited, "blue = \"#ff00ff\"\n")
	if err := os.Remove(filepath.Join(dir, "nord.toml")); err != nil {
		t.Fatal(err)
	}
	if _, err := Load("testapp", ""); err != nil {
		t.Fatal(err)
	}
	if body, err := os.ReadFile(edited); err != nil || !strings.Contains(string(body), "#ff00ff") {
		t.Errorf("an edited theme was overwritten: %q %v", body, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "nord.toml")); err != nil {
		t.Errorf("a missing theme should be written back: %v", err)
	}
}

func TestUnknownThemeNamesWhatIsOnOffer(t *testing.T) {
	dir := tempConfig(t)
	write(t, filepath.Join(dir, "mine.toml"), "blue = \"blue\"\n")

	_, err := Load("testapp", "nosuchtheme")
	if err == nil {
		t.Fatal("an unknown theme should error")
	}
	for _, want := range []string{"nosuchtheme", "mine", "default", "gruvbox-material"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should mention %q", err, want)
		}
	}
}

func TestBadThemeFilesError(t *testing.T) {
	dir := tempConfig(t)
	write(t, filepath.Join(dir, "typo.toml"), "blu = \"blue\"\n")
	write(t, filepath.Join(dir, "junk.toml"), "blue = \"puce\"\n")
	write(t, filepath.Join(dir, "broken.toml"), "blue =\n")

	for _, tc := range []struct{ name, want string }{
		{"typo", "unknown key"},
		{"junk", "unknown colour"},
		{"broken", "broken.toml"},
	} {
		_, err := Load("testapp", tc.name)
		if err == nil {
			t.Errorf("%s should not load", tc.name)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %q should mention %q", tc.name, err, tc.want)
		}
	}
}

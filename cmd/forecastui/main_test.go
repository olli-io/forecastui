package main

import (
	"flag"
	"io"
	"strings"
	"testing"

	"github.com/olli-io/forecastui/internal/geo"
	"github.com/olli-io/forecastui/internal/theme"
)

// A flag may stand after a positional argument, which the flag package does
// not allow on its own.
func TestPositionalReadsFlagsOnEitherSide(t *testing.T) {
	for _, tc := range []struct {
		argv  []string
		want  []string
		place string
		once  bool
	}{
		{argv: []string{"--once", "day", "--place", "turku"},
			want: []string{"day"}, place: "turku", once: true},
		{argv: []string{"--once", "--place", "tampere", "day"},
			want: []string{"day"}, place: "tampere", once: true},
		{argv: []string{"week"}, want: []string{"week"}},
		{argv: []string{"48", "24.94", "60.17"},
			want: []string{"48", "24.94", "60.17"}},
		// A western longitude opens with a minus: a coordinate, not a flag.
		{argv: []string{"--once", "12", "-24.94", "60.17"},
			want: []string{"12", "-24.94", "60.17"}, once: true},
		{argv: nil, want: nil},
	} {
		fs := flag.NewFlagSet("forecastui", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		once := fs.Bool("once", false, "")
		place := fs.String("place", "", "")
		if err := fs.Parse(tc.argv); err != nil {
			t.Fatalf("%v: %v", tc.argv, err)
		}

		got := positional(fs)
		if strings.Join(got, " ") != strings.Join(tc.want, " ") {
			t.Errorf("%v: positional %q, want %q", tc.argv, got, tc.want)
		}
		if *place != tc.place {
			t.Errorf("%v: place %q, want %q", tc.argv, *place, tc.place)
		}
		if *once != tc.once {
			t.Errorf("%v: once %v, want %v", tc.argv, *once, tc.once)
		}
	}
}

// An install lands with the fallback theme pinned in config.toml; a config
// already on disk is left alone, even one that names no theme.
func TestPinThemeWritesTheFallbackIntoAMissingConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cfg, _ := geo.Load(appName)
	pinTheme(cfg)
	if cfg.Theme != theme.FallbackName {
		t.Fatalf("theme is %q after the pin, want %q", cfg.Theme, theme.FallbackName)
	}
	again, err := geo.Load(appName)
	if err != nil {
		t.Fatalf("reading the config back: %v", err)
	}
	if again.Theme != theme.FallbackName {
		t.Errorf("config.toml pins %q, want %q", again.Theme, theme.FallbackName)
	}

	// A config whose theme line was removed stays bare: the pin is for the
	// first run, not a setting that cannot be deleted.
	again.Theme = ""
	if err := again.Save(appName); err != nil {
		t.Fatal(err)
	}
	cfg, _ = geo.Load(appName)
	pinTheme(cfg)
	if cfg.Theme != "" {
		t.Errorf("a bare config was re-pinned to %q", cfg.Theme)
	}
}

// The theme is chosen by flag, then environment, then config.
func TestFirstPrefersTheFlagThenTheEnvironment(t *testing.T) {
	for _, tc := range []struct {
		vals []string
		want string
	}{
		{[]string{"nord", "gruvbox-material", "default"}, "nord"},
		{[]string{"", "gruvbox-material", "default"}, "gruvbox-material"},
		{[]string{"", "", "default"}, "default"},
		{[]string{"", "", ""}, ""},
	} {
		if got := first(tc.vals...); got != tc.want {
			t.Errorf("first%v is %q, want %q", tc.vals, got, tc.want)
		}
	}
}

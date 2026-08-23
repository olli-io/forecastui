package ui

import (
	"strings"
	"testing"

	"github.com/olli-io/forecastui/internal/fmi"
	"github.com/olli-io/forecastui/internal/render"
)

func TestParseGlyphMode(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want GlyphMode
		bad  bool
	}{
		{"", GlyphAuto, false},
		{"auto", GlyphAuto, false},
		{"Nerd", GlyphNerd, false},
		{"1", GlyphNerd, false},
		{"plain", GlyphPlain, false},
		{"ascii", GlyphPlain, false},
		{"0", GlyphPlain, false},
		{"maybe", GlyphAuto, true},
	} {
		got, err := ParseGlyphMode(tc.in)
		if (err != nil) != tc.bad {
			t.Errorf("ParseGlyphMode(%q) error = %v, want error: %v", tc.in, err, tc.bad)
		}
		if got != tc.want {
			t.Errorf("ParseGlyphMode(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// The flag and the environment variable are the two ways to settle the
// question outright, and neither may reach for the terminal.
func TestGlyphModeOverridesTheChecks(t *testing.T) {
	t.Setenv(glyphEnv, "plain")
	if !NerdFont(GlyphNerd) {
		t.Error("--glyphs nerd must win over the environment")
	}
	if NerdFont(GlyphPlain) {
		t.Error("--glyphs plain must win over the environment")
	}
	if NerdFont(GlyphAuto) {
		t.Error("the environment must settle it when the flag says auto")
	}
	t.Setenv(glyphEnv, "nerd")
	if !NerdFont(GlyphAuto) {
		t.Error("the environment must settle it the other way too")
	}
}

// A console with a built-in font is the one case that needs no probing.
func TestBuiltInConsoleFontsGetASCII(t *testing.T) {
	t.Setenv(glyphEnv, "")
	for _, term := range []string{"linux", "dumb", ""} {
		t.Setenv("TERM", term)
		if NerdFont(GlyphAuto) {
			t.Errorf("TERM=%q should fall back to ASCII", term)
		}
	}
}

func TestNerdName(t *testing.T) {
	for _, tc := range []struct {
		name string
		want bool
	}{
		{"JetBrainsMono Nerd Font Mono", true},
		{"CaskaydiaMono NFM", true},
		{"Symbols Nerd Font", true},
		{"xft:Hack:size=11", false},
		{"-misc-fixed-medium-r-normal--13-120-75-75-c-70-iso10646-1", false},
		{"Info Display", false},
	} {
		if got := nerdName(tc.name); got != tc.want {
			t.Errorf("nerdName(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestCursorColAndFontReply(t *testing.T) {
	reply := "\x1b]50;JetBrainsMono Nerd Font\x1b\\\x1b[7;2R"
	if got, ok := cursorCol(reply); !ok || got != 2 {
		t.Errorf("cursorCol = %d, %v; want 2, true", got, ok)
	}
	if got := fontReply(reply); got != "JetBrainsMono Nerd Font" {
		t.Errorf("fontReply = %q", got)
	}
	if _, ok := cursorCol("no reply at all"); ok {
		t.Error("a reply with no report should not parse")
	}
	if got := fontReply("\x1b[7;2R"); got != "" {
		t.Errorf("a terminal that ignored the font query reported %q", got)
	}
}

// Without a Nerd Font the sky row must still be one cell a column, or the
// braille grid under it shears.
func TestPlainSkyRowKeepsTheGrid(t *testing.T) {
	a := newTestApp(t, 120, 30, 48)
	nerd := render.Plain(render.Sky(a.cols, render.Opts{Count: 12, Nerd: true}))
	plain := render.Plain(render.Sky(a.cols, render.Opts{Count: 12, Nerd: false}))
	if lipglossWidth(nerd) != lipglossWidth(plain) {
		t.Errorf("sky row is %d wide with glyphs and %d without:\n%s\n%s",
			lipglossWidth(nerd), lipglossWidth(plain), nerd, plain)
	}
	if strings.ContainsRune(plain, fmi.SampleGlyph) {
		t.Error("the plain row still carries a Nerd Font glyph")
	}
	if strings.TrimSpace(plain) == "" {
		t.Error("the plain row drew nothing at all")
	}
}

// Every symbol needs both forms, and both have to be one cell wide.
func TestEverySymbolHasBothForms(t *testing.T) {
	for code := 0; code <= 100; code++ {
		for _, night := range []bool{false, true} {
			s := fmi.Describe(code, night)
			for _, r := range []rune{s.Rune(true), s.Rune(false)} {
				if r == 0 {
					t.Fatalf("symbol %d (night %v) has no glyph", code, night)
				}
				if w := lipglossWidth(string(r)); w != 1 {
					t.Errorf("symbol %d (night %v): %q is %d cells wide", code, night, r, w)
				}
			}
		}
	}
}

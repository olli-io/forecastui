package fmi

import (
	"testing"

	"charm.land/lipgloss/v2"
)

// A double-width glyph would shear the whole strip, so every symbol must
// measure exactly one cell — the night forms too.
func TestSymbolGlyphsAreSingleWidth(t *testing.T) {
	for code := range symbols {
		for _, night := range []bool{false, true} {
			s := Describe(code, night)
			if w := lipgloss.Width(string(s.Glyph)); w != 1 {
				t.Errorf("code %d (%s) night=%v: glyph %q is %d cells wide",
					code, s.Desc, night, s.Glyph, w)
			}
		}
	}
}

func TestDescribeUnknownIsBlank(t *testing.T) {
	for _, code := range []int{0, 4, 999, -1} {
		if s := Describe(code, false); s.Glyph != ' ' || s.Desc != "" {
			t.Errorf("code %d: got %q/%q, want a blank", code, s.Glyph, s.Desc)
		}
	}
}

func TestDescribeKnownCodes(t *testing.T) {
	if got := Describe(1, false).Desc; got != "clear" {
		t.Errorf("code 1: %q", got)
	}
	if got := Describe(92, false).Desc; got != "fog" {
		t.Errorf("code 92: %q", got)
	}
}

// After dark a clear sky is a moon and rain is still rain: only the glyph
// changes, and only where there is a night form.
func TestNightGlyphsOnlyChangeTheSky(t *testing.T) {
	for code, s := range symbols {
		night := Describe(code, true)
		if night.Desc != s.Desc {
			t.Errorf("code %d: night description %q, want %q", code, night.Desc, s.Desc)
		}
		_, hasNight := afterDark[code]
		if changed := night.Glyph != s.Glyph; changed != hasNight {
			t.Errorf("code %d (%s): glyph changed=%v, want %v", code, s.Desc, changed, hasNight)
		}
	}
	if Describe(1, true).Glyph != nfNight {
		t.Error("a clear night should be a moon")
	}
	if Describe(2, true).Glyph != nfNightPartlyCloud {
		t.Error("a partly cloudy night should be a moon behind cloud")
	}
}

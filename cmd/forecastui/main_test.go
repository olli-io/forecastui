package main

import (
	"flag"
	"io"
	"strings"
	"testing"
)

// A flag is allowed to stand after a positional argument as well as before it,
// which the flag package does not do on its own.
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
		// A western longitude opens with a minus: it is a coordinate, not a
		// flag, and neither is anything after it.
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

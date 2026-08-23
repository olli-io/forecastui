// Command forecastui shows the Finnish Meteorological Institute's forecast as
// an interactive terminal dashboard, or as a one-shot chart with --once.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/olli-io/forecastui/internal/cache"
	"github.com/olli-io/forecastui/internal/fmi"
	"github.com/olli-io/forecastui/internal/geo"
	"github.com/olli-io/forecastui/internal/ui"
)

const appName = "forecastui"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, appName+": "+err.Error())
		os.Exit(1)
	}
}

func run() error {
	var (
		once   = flag.Bool("once", false, "print one chart and exit")
		hours  = flag.Int("hours", 0, "hours of forecast to show (default 48, or 168 with -week)")
		week   = flag.Bool("week", false, "show a week, aggregated to 3 h steps")
		place  = flag.String("place", "", "place name to look up instead of coordinates")
		lat    = flag.Float64("lat", 0, "latitude")
		lon    = flag.Float64("lon", 0, "longitude")
		width  = flag.Int("width", 0, "output width for -once (default: terminal width)")
		glyphs = flag.String("glyphs", "auto",
			"weather symbols: auto, nerd (Nerd Font glyphs) or plain (ASCII)")
	)
	flag.Usage = usage
	flag.Parse()

	loc, span, err := resolve(positional(flag.CommandLine), *place, *lat, *lon, *hours, *week)
	if err != nil {
		return err
	}

	mode, err := ui.ParseGlyphMode(*glyphs)
	if err != nil {
		return err
	}

	// The font check probes the terminal, so it is made here rather than in
	// the model: by the time Bubble Tea is running, the screen is not ours to
	// write a test glyph onto. A one-shot chart draws the same sky row, so it
	// asks the same question first.
	nerd := ui.NerdFont(mode)

	if *once {
		return runOnce(loc, span, *width, nerd)
	}

	cfg, _ := geo.Load(appName) // a missing or broken config is not fatal
	m := ui.New(loc, span, cfg, nerd)
	_, err = tea.NewProgram(m).Run()
	return err
}

// positional collects the arguments that are not flags, letting a flag stand
// after one as well as before it. Go's flag package stops at the first word it
// does not recognise, so "forecastui --once day --place turku" would leave
// "--place turku" to be read as a longitude and a latitude. Each positional is
// set aside and what follows it parsed again.
func positional(fs *flag.FlagSet) []string {
	var args []string
	for rest := fs.Args(); len(rest) > 0; rest = fs.Args() {
		args = append(args, rest[0])
		// A western longitude opens with a minus and would be taken for a
		// flag, so the coordinates end the interleaving: everything from one
		// on is positional.
		if len(rest) > 1 && negative(rest[1]) {
			return append(args, rest[1:]...)
		}
		_ = fs.Parse(rest[1:]) // the caller's set decides what an error does
	}
	return args
}

// negative reports whether an argument is a negative number rather than a flag.
func negative(s string) bool {
	if !strings.HasPrefix(s, "-") {
		return false
	}
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

func usage() {
	fmt.Fprintf(os.Stderr, `usage: %s [flags] [day|week|hours] [lon lat]

Interactive by default. The positional form is kept for compatibility with the
original turku-forecast.sh:

  %s day           the next two days, hour by hour
  %s week          a week, 3 h steps from now
  %s 24            24 hours at the default location
  %s 48 24.94 60.17   48 hours at a given lon/lat

flags:
`, appName, appName, appName, appName, appName)
	flag.PrintDefaults()
}

// defaultPlace is Turku; only these exact coordinates are labelled by name.
var defaultPlace = geo.Place{Name: "Turku", Lat: 60.4518, Lon: 22.2666}

// resolve merges flags, positional arguments and config into a location and a
// span. Positional arguments mirror the legacy script: [hours|week] [lon lat].
func resolve(args []string, place string, lat, lon float64, hours int, week bool) (geo.Place, ui.Span, error) {
	if len(args) > 0 {
		switch args[0] {
		case "week", "w":
			week = true
		case "day", "d":
			// The two named ranges the interactive view tabs between, so the
			// positional form reaches the same two by the same names.
			hours = 48
		default:
			n, err := strconv.Atoi(args[0])
			if err != nil || n < 1 {
				return geo.Place{}, ui.Span{}, fmt.Errorf(
					"want day, week or a positive number of hours, got %q", args[0])
			}
			hours = n
		}
	}
	if len(args) >= 3 {
		var err error
		if lon, err = strconv.ParseFloat(args[1], 64); err != nil {
			return geo.Place{}, ui.Span{}, fmt.Errorf("bad longitude %q", args[1])
		}
		if lat, err = strconv.ParseFloat(args[2], 64); err != nil {
			return geo.Place{}, ui.Span{}, fmt.Errorf("bad latitude %q", args[2])
		}
	}

	span := ui.Span{Hours: hours, Slots: week}
	if week && hours == 0 {
		span.Hours = 168
	}
	if span.Hours == 0 {
		span.Hours = 48
	}

	switch {
	case place != "":
		p, err := geo.Search(context.Background(), appName, place, 1)
		if err != nil {
			return geo.Place{}, span, err
		}
		if len(p) == 0 {
			return geo.Place{}, span, fmt.Errorf("no place found for %q", place)
		}
		return p[0], span, nil
	case lat != 0 || lon != 0:
		return geo.Place{Name: fmt.Sprintf("%g, %g", lat, lon), Lat: lat, Lon: lon}, span, nil
	}

	if cfg, err := geo.Load(appName); err == nil && cfg.Default != nil {
		return *cfg.Default, span, nil
	}
	return defaultPlace, span, nil
}

// runOnce prints a single chart, the way the shell script did.
func runOnce(p geo.Place, span ui.Span, width int, nerd bool) error {
	hours, err := onceHours(p, span)
	if err != nil {
		if errors.Is(err, fmi.ErrNoData) {
			return errors.New("no forecast data returned for that location")
		}
		return err
	}
	out, err := ui.Once(p, hours, span, width, nerd)
	if err != nil {
		return err
	}
	fmt.Println(out)
	return nil
}

// onceHours is the forecast a one-shot chart draws. A run inside the refresh
// window is answered from the cache: FMI publishes hourly, so a chart piped
// into a prompt or a status bar every few seconds would otherwise ask them the
// same question over and over for the same answer.
func onceHours(p geo.Place, span ui.Span) ([]fmi.Hour, error) {
	if hours, at, err := cache.Load(appName, p.Lat, p.Lon, span.Hours); err == nil &&
		len(hours) > 0 && cache.Fresh(at) {
		return hours, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	from := time.Now().UTC().Truncate(time.Hour)
	// The range is inclusive of both ends, so asking for 24 gives 24 bars.
	to := from.Add(time.Duration(span.Hours-1) * time.Hour)

	hours, err := fmi.NewClient().Fetch(ctx, p.Lat, p.Lon, from, to)
	if err != nil {
		return nil, err
	}
	cache.Save(appName, p.Lat, p.Lon, span.Hours, hours) // a failed save is not fatal
	return hours, nil
}

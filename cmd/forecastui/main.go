// Command forecastui shows the FMI forecast as an interactive terminal
// dashboard, or as a one-shot chart with --once.
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
	"github.com/olli-io/forecastui/internal/theme"
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
		themeName = flag.String("theme", "",
			"colour theme: a name from the themes directory, or a path to a .toml file")
	)
	flag.Usage = usage
	flag.Parse()

	cfg, _ := geo.Load(appName) // a missing or broken config is not fatal
	pinTheme(cfg)

	// The flag beats the environment beats the config, as everywhere else.
	t, err := theme.Load(appName, first(*themeName, os.Getenv("FORECASTUI_THEME"), cfg.Theme))
	if err != nil {
		return err
	}
	ui.Use(t)

	loc, span, err := resolve(positional(flag.CommandLine), cfg, *place, *lat, *lon, *hours, *week)
	if err != nil {
		return err
	}

	mode, err := ui.ParseGlyphMode(*glyphs)
	if err != nil {
		return err
	}

	// Probes the terminal, so it must run before Bubble Tea owns the screen.
	nerd := ui.NerdFont(mode)

	if *once {
		return runOnce(loc, span, *width, nerd)
	}

	m := ui.New(loc, span, cfg, nerd)
	_, err = tea.NewProgram(m).Run()
	return err
}

// pinTheme names the fallback theme in a config.toml that does not exist yet,
// so an install lands with its theme written where it will be found and
// changed, rather than implied by the binary. A config already on disk is left
// alone, themed or not.
func pinTheme(cfg *geo.Config) {
	path, err := geo.ConfigPath(appName)
	if err != nil {
		return
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return
	}
	cfg.Theme = theme.FallbackName
	_ = cfg.Save(appName) // best effort: the fallback applies either way
}

// first returns the first value that was set.
func first(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// positional collects non-flag arguments, allowing flags to follow them.
// Go's flag package stops at the first unrecognised word, so each positional
// is set aside and what follows it parsed again.
func positional(fs *flag.FlagSet) []string {
	var args []string
	for rest := fs.Args(); len(rest) > 0; rest = fs.Args() {
		args = append(args, rest[0])
		// A western longitude opens with a minus and would look like a flag,
		// so everything from the coordinates on is positional.
		if len(rest) > 1 && negative(rest[1]) {
			return append(args, rest[1:]...)
		}
		_ = fs.Parse(rest[1:])
	}
	return args
}

// negative reports whether an argument is a negative number, not a flag.
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

var defaultPlace = geo.Place{Name: "Turku", Lat: 60.4518, Lon: 22.2666}

// resolve merges flags, positional arguments ([hours|week] [lon lat]) and
// config into a location and a span.
func resolve(args []string, cfg *geo.Config, place string, lat, lon float64, hours int, week bool) (geo.Place, ui.Span, error) {
	if len(args) > 0 {
		switch args[0] {
		case "week", "w":
			week = true
		case "day", "d":
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

	if cfg != nil && cfg.Default != nil {
		return *cfg.Default, span, nil
	}
	return defaultPlace, span, nil
}

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

// onceHours serves from cache while fresh: FMI publishes hourly, so a chart
// piped into a prompt every few seconds must not refetch each time.
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

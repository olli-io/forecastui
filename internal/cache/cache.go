// Package cache stores the last good forecast per location, so the UI can
// show something at startup and survive a lost network.
package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/olli-io/forecastui/internal/fmi"
)

type entry struct {
	Fetched time.Time  `json:"fetched"`
	Hours   []fmi.Hour `json:"hours"`
}

// maxAge is how long a cached forecast is worth showing at all.
const maxAge = 24 * time.Hour

// Current is how long a saved forecast counts as current; FMI publishes hourly.
const Current = 10 * time.Minute

// Fresh reports whether a forecast fetched at t still counts as current.
func Fresh(t time.Time) bool { return !t.IsZero() && time.Since(t) < Current }

// path keys an entry by location and span: two days of hours cannot answer a
// request for the week, so spans are kept apart.
func path(app string, lat, lon float64, span int) (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	name := fmt.Sprintf("%.4f_%.4f_%dh.json", lat, lon, span)
	return filepath.Join(dir, app, name), nil
}

// Save writes a forecast to the cache.
func Save(app string, lat, lon float64, span int, hours []fmi.Hour) error {
	p, err := path(app, lat, lon, span)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	body, err := json.Marshal(entry{Fetched: time.Now(), Hours: hours})
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// Load returns the cached forecast and when it was fetched. A stale entry
// reads as empty.
func Load(app string, lat, lon float64, span int) ([]fmi.Hour, time.Time, error) {
	p, err := path(app, lat, lon, span)
	if err != nil {
		return nil, time.Time{}, err
	}
	body, err := os.ReadFile(p)
	if err != nil {
		return nil, time.Time{}, err
	}
	var e entry
	if err := json.Unmarshal(body, &e); err != nil {
		return nil, time.Time{}, err
	}
	if time.Since(e.Fetched) > maxAge {
		return nil, e.Fetched, nil
	}
	return e.Hours, e.Fetched, nil
}

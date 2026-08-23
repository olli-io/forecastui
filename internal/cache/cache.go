// Package cache stores the last good forecast per location, so the UI can
// show something the moment it starts and survive a lost network.
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

// maxAge is how long a cached forecast is worth showing at all. Beyond a day
// the bars would be describing weather that has already happened.
const maxAge = 24 * time.Hour

func path(app string, lat, lon float64) (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, app, fmt.Sprintf("%.4f_%.4f.json", lat, lon)), nil
}

// Save writes a forecast to the cache. Failures are reported but never fatal:
// a working cache is a convenience, not a requirement.
func Save(app string, lat, lon float64, hours []fmi.Hour) error {
	p, err := path(app, lat, lon)
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

// Load returns the cached forecast and when it was fetched. A missing, broken
// or stale entry reads as empty rather than as an error worth showing.
func Load(app string, lat, lon float64) ([]fmi.Hour, time.Time, error) {
	p, err := path(app, lat, lon)
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

package geo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Photon is OpenStreetMap data with an autocomplete index in front of it: it
// treats the last term as a prefix, where Nominatim matches only whole words.
const photon = "https://photon.komoot.io/api/"

// Backstop behind the keystroke debounce, so Photon is not hammered in bulk.
var (
	rateMu   sync.Mutex
	lastCall time.Time
)

const minInterval = 250 * time.Millisecond

var httpClient = &http.Client{Timeout: 10 * time.Second}

type feature struct {
	Geometry struct {
		Coordinates []float64 `json:"coordinates"` // lon, lat, in that order
	} `json:"geometry"`
	Properties props `json:"properties"`
}

type props struct {
	Name        string `json:"name"`
	HouseNumber string `json:"housenumber"`
	Street      string `json:"street"`
	City        string `json:"city"`
	County      string `json:"county"`
	State       string `json:"state"`
	Country     string `json:"country"`
}

// Search looks up a place by name. It blocks for the rate limit, so callers
// should run it off the UI goroutine.
func Search(ctx context.Context, app, query string, limit int) ([]Place, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 8
	}
	if err := wait(ctx); err != nil {
		return nil, err
	}

	q := url.Values{}
	q.Set("q", query)
	// Duplicate labels are dropped below, so ask for more than will be shown.
	q.Set("limit", strconv.Itoa(limit*2))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, photon+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", app+" (terminal weather client)")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("place search: %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	var found struct {
		Features []feature `json:"features"`
	}
	if err := json.Unmarshal(body, &found); err != nil {
		return nil, fmt.Errorf("place search: %w", err)
	}
	return collect(found.Features, limit), nil
}

// collect turns hits into places, dropping ones whose label is already listed.
// Photon returns the town, the parish and the station all under one name.
func collect(features []feature, limit int) []Place {
	places := make([]Place, 0, limit)
	seen := make(map[string]bool, limit)
	for _, f := range features {
		if len(places) == limit {
			break
		}
		c := f.Geometry.Coordinates
		name := label(f.Properties)
		if len(c) < 2 || name == "" || seen[name] {
			continue
		}
		seen[name] = true
		places = append(places, Place{Name: name, Lon: c[0], Lat: c[1]})
	}
	return places
}

// label names a hit: the place, then just enough to tell it from another of
// the same name — its town, county or state, and the country.
func label(p props) string {
	name := p.Name
	if name == "" {
		name = strings.TrimSpace(p.HouseNumber + " " + p.Street)
	}
	if name == "" {
		return ""
	}
	parts := []string{name}
	for _, s := range []string{p.City, p.County, p.State} {
		if s != "" && !slices.Contains(parts, s) {
			parts = append(parts, s)
			break // one line of context is enough; the country follows
		}
	}
	if p.Country != "" && !slices.Contains(parts, p.Country) {
		parts = append(parts, p.Country)
	}
	return strings.Join(parts, ", ")
}

// wait keeps requests from going out back to back.
func wait(ctx context.Context) error {
	rateMu.Lock()
	delay := minInterval - time.Since(lastCall)
	lastCall = time.Now().Add(max(0, delay))
	rateMu.Unlock()

	if delay <= 0 {
		return nil
	}
	t := time.NewTimer(delay)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

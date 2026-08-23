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

// Photon is OpenStreetMap's data — the same Nominatim draws on — with an
// autocomplete index in front of it, which is what a search box actually
// needs. Nominatim matches whole words, so "Kuopio" finds Kuopio and nothing
// else, while half the places anyone is looking for carry the name in the
// genitive: Kuopion lentoasema, Kuopion tuomiokirkko. Photon treats the last
// term as a prefix, so a name typed part of the way still finds them.
const photon = "https://photon.komoot.io/api/"

// Photon asks only that it not be hammered in bulk. The search is debounced a
// keystroke's pause already; this is the backstop, and it is short enough that
// a query still lands while the finger is off the key.
var (
	rateMu   sync.Mutex
	lastCall time.Time
)

const minInterval = 250 * time.Millisecond

var httpClient = &http.Client{Timeout: 10 * time.Second}

// feature is one hit: a point, and the administrative fields around it.
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

// Search looks up a place by name. It blocks as needed to respect the
// upstream rate limit, so callers should run it off the UI goroutine.
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
	// Names that collapse to the same label are dropped below, so ask for
	// more than will be shown: a town and the station named after it come
	// back as separate hits and only one of them survives.
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

// collect turns the hits into places, dropping the ones that would read the
// same as a place already on the list. Photon returns the town, the historic
// parish and the railway station all under one name; three identical rows
// pointing at three coordinates a few hundred metres apart is not a choice
// anyone can make.
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

// label names a hit: the place itself, then just enough to tell it from
// another of the same name — the town it stands in, or failing that the
// region, and the country. Photon hands these back as separate fields rather
// than as Nominatim's full administrative chain, so there is nothing to trim.
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

// wait keeps the app from sending requests back to back.
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

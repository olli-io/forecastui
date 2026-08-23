package fmi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// pal_skandinavia is FMI's edited forecast, not a raw model run.
const endpoint = "https://opendata.fmi.fi/edr/collections/pal_skandinavia/position"

const params = "temperature,windspeedms,hourlymaximumgust,winddirection," +
	"precipitation1h,pop,totalcloudcover,humidity,weathersymbol3"

// ErrNoData means the request succeeded but covered no forecast hours.
var ErrNoData = errors.New("fmi: no forecast data returned")

// Client fetches forecasts. The zero value is not usable; use NewClient.
type Client struct {
	HTTP *http.Client
}

func NewClient() *Client {
	return &Client{HTTP: &http.Client{Timeout: 15 * time.Second}}
}

// Fetch returns hourly forecast steps covering [from, to] at the given point.
func (c *Client) Fetch(ctx context.Context, lat, lon float64, from, to time.Time) ([]Hour, error) {
	q := url.Values{}
	q.Set("coords", fmt.Sprintf("POINT(%g %g)", lon, lat))
	q.Set("parameter-name", params)
	q.Set("datetime", from.UTC().Format(stamp)+"/"+to.UTC().Format(stamp))
	q.Set("f", "GeoJSON")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fmi: %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, err
	}
	return Parse(body)
}

const stamp = "2006-01-02T15:04:05Z"

// maxBody caps the response read; two weeks of parameters is well under a MB.
const maxBody = 16 << 20

// feature mirrors one GeoJSON feature. EDR splits each parameter into its own
// feature with its own time array, so they must be merged back together.
type feature struct {
	Properties map[string]json.RawMessage `json:"properties"`
}

type collection struct {
	Features []feature `json:"features"`
	Detail   string    `json:"detail"`
}

// Parse merges an EDR GeoJSON response into ordered hourly steps.
func Parse(body []byte) ([]Hour, error) {
	var col collection
	if err := json.Unmarshal(body, &col); err != nil {
		return nil, fmt.Errorf("fmi: decode: %w", err)
	}
	if col.Detail != "" && len(col.Features) == 0 {
		return nil, fmt.Errorf("fmi: %s", col.Detail)
	}

	// param name -> timestamp -> value; nil stays nil to distinguish gaps from 0.
	merged := map[string]map[time.Time]*float64{}
	seen := map[time.Time]bool{}

	for _, f := range col.Features {
		raw, ok := f.Properties["time"]
		if !ok {
			continue
		}
		var stamps []string
		if err := json.Unmarshal(raw, &stamps); err != nil {
			return nil, fmt.Errorf("fmi: decode times: %w", err)
		}
		times := make([]time.Time, len(stamps))
		for i, s := range stamps {
			t, err := time.Parse(stamp, s)
			if err != nil {
				return nil, fmt.Errorf("fmi: bad timestamp %q: %w", s, err)
			}
			times[i] = t
			seen[t] = true
		}

		for key, rawVals := range f.Properties {
			if key == "time" {
				continue
			}
			var vals []*float64
			if err := json.Unmarshal(rawVals, &vals); err != nil {
				// Non-numeric parameters are not rendered.
				continue
			}
			name := strings.ToLower(key)
			if merged[name] == nil {
				merged[name] = map[time.Time]*float64{}
			}
			for i, v := range vals {
				if i >= len(times) {
					break
				}
				merged[name][times[i]] = v
			}
		}
	}

	if len(seen) == 0 {
		return nil, ErrNoData
	}

	order := make([]time.Time, 0, len(seen))
	for t := range seen {
		order = append(order, t)
	}
	sort.Slice(order, func(i, j int) bool { return order[i].Before(order[j]) })

	get := func(name string, t time.Time) Val {
		m, ok := merged[name]
		if !ok {
			return Val{}
		}
		v, ok := m[t]
		if !ok || v == nil {
			return Val{}
		}
		return Val{V: *v, OK: true}
	}

	hours := make([]Hour, 0, len(order))
	for _, t := range order {
		h := Hour{
			Time:  t,
			Temp:  get("temperature", t),
			Wind:  get("windspeedms", t),
			Gust:  get("hourlymaximumgust", t),
			Dir:   get("winddirection", t),
			Rain:  get("precipitation1h", t),
			POP:   get("pop", t),
			Cloud: get("totalcloudcover", t),
			Hum:   get("humidity", t),
		}
		if s := get("weathersymbol3", t); s.OK {
			h.Sym = int(s.V)
		}
		hours = append(hours, h)
	}
	return hours, nil
}

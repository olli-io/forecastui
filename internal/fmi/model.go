// Package fmi fetches forecasts from the Finnish Meteorological Institute's
// open data service (OGC API-EDR, no API key required).
package fmi

import "time"

// Val is a forecast value that may be absent; the API returns null for gaps,
// and a missing temperature must not read as 0 °C.
type Val struct {
	V  float64
	OK bool
}

// Or returns the value, or def when absent.
func (v Val) Or(def float64) float64 {
	if !v.OK {
		return def
	}
	return v.V
}

// Hour is one hourly step of the forecast.
type Hour struct {
	Time  time.Time // UTC; render in time.Local
	Temp  Val       // °C
	Wind  Val       // m/s
	Gust  Val       // m/s, hourly maximum
	Dir   Val       // degrees the wind blows *from*
	Rain  Val       // mm/h
	POP   Val       // probability of precipitation, %
	Cloud Val       // total cloud cover, %
	Hum   Val       // relative humidity, %
	Sym   int       // weathersymbol3 code, 0 when absent
}

// Forecast is a location's forecast as of Fetched.
type Forecast struct {
	Place   string
	Lat     float64
	Lon     float64
	Hours   []Hour
	Fetched time.Time
}

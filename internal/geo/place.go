// Package geo resolves place names to coordinates and stores the user's
// default location and favourites.
package geo

import "fmt"

// Place is a named point.
type Place struct {
	Name string  `json:"name"`
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
}

// Label is how a place is shown in the UI.
func (p Place) Label() string {
	if p.Name != "" {
		return p.Name
	}
	return fmt.Sprintf("%.4f, %.4f", p.Lat, p.Lon)
}

// Same reports whether two places refer to the same point, within the
// precision the forecast grid can distinguish.
func (p Place) Same(q Place) bool {
	const eps = 1e-4
	return abs(p.Lat-q.Lat) < eps && abs(p.Lon-q.Lon) < eps
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

package sun

import (
	"testing"
	"time"
)

// Turku, and Utsjoki for the polar cases.
const (
	turkuLat, turkuLon     = 60.4518, 22.2666
	laplandLat, laplandLon = 69.9083, 27.0286
)

func helsinki(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Helsinki")
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}
	return loc
}

func TestNightFollowsTheSeason(t *testing.T) {
	loc := helsinki(t)
	for _, tc := range []struct {
		name  string
		when  time.Time
		lat   float64
		lon   float64
		night bool
	}{
		// Midsummer in Turku: light well past ten in the evening.
		{"midsummer noon", time.Date(2026, 6, 21, 12, 0, 0, 0, loc), turkuLat, turkuLon, false},
		{"midsummer 22:00", time.Date(2026, 6, 21, 22, 0, 0, 0, loc), turkuLat, turkuLon, false},
		{"midsummer 01:00", time.Date(2026, 6, 22, 1, 0, 0, 0, loc), turkuLat, turkuLon, true},
		// Midwinter: the sun clears the horizon at noon, gone before five.
		{"midwinter noon", time.Date(2026, 12, 21, 12, 0, 0, 0, loc), turkuLat, turkuLon, false},
		{"midwinter 16:00", time.Date(2026, 12, 21, 16, 0, 0, 0, loc), turkuLat, turkuLon, true},
		{"midwinter 08:00", time.Date(2026, 12, 21, 8, 0, 0, 0, loc), turkuLat, turkuLon, true},
		// Above the arctic circle the sun does not set in June, nor rise in
		// December.
		{"polar day, midnight", time.Date(2026, 6, 21, 0, 0, 0, 0, loc), laplandLat, laplandLon, false},
		{"polar night, noon", time.Date(2026, 12, 21, 12, 0, 0, 0, loc), laplandLat, laplandLon, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Night(tc.when, tc.lat, tc.lon); got != tc.night {
				t.Errorf("night=%v, want %v (elevation %.2f°)",
					got, tc.night, Elevation(tc.when, tc.lat, tc.lon))
			}
		})
	}
}

// The equinox sun crosses the horizon within minutes of six, solar time,
// wherever you stand.
func TestEquinoxSunriseIsNearSix(t *testing.T) {
	loc := helsinki(t)
	// Turku is 22.27° east, so solar noon runs about 90 min early: sunrise
	// lands near 07:30 local in late March.
	before := time.Date(2026, 3, 20, 6, 30, 0, 0, loc)
	after := time.Date(2026, 3, 20, 8, 30, 0, 0, loc)
	if !Night(before, turkuLat, turkuLon) {
		t.Errorf("06:30 should still be dark: %.2f°", Elevation(before, turkuLat, turkuLon))
	}
	if Night(after, turkuLat, turkuLon) {
		t.Errorf("08:30 should be light: %.2f°", Elevation(after, turkuLat, turkuLon))
	}
}

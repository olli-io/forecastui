// Package sun answers one question: is the sun up? The forecast's weather
// codes say nothing about the time of day, so a clear hour is drawn with a sun
// or a moon depending on this, and at these latitudes a fixed "evening starts
// at eight" would be wrong half the year — Turku has daylight past eleven in
// June and none after four in December.
package sun

import (
	"math"
	"time"
)

// horizon is the elevation the sun's centre sits at when its upper limb
// touches the horizon: half a degree of disc, plus the refraction that lifts
// it into view slightly before it is really there. It is the same threshold
// almanacs use for sunrise and sunset.
const horizon = -0.833

// Night reports whether the sun is below the horizon at the given place.
func Night(t time.Time, lat, lon float64) bool {
	return Elevation(t, lat, lon) < horizon
}

// Elevation is the sun's angle above the horizon, in degrees. It follows the
// low-precision solar position algorithm from the Astronomical Almanac, good
// to about a hundredth of a degree either side of 2000 — far tighter than the
// hour the forecast is bucketed into.
func Elevation(t time.Time, lat, lon float64) float64 {
	// Days since J2000.0, the epoch every term below is anchored to.
	n := float64(t.UTC().UnixNano())/(24*60*60*1e9) + 2440587.5 - 2451545.0

	mean := rad(280.460 + 0.9856474*n) // mean longitude
	anom := rad(357.528 + 0.9856003*n) // mean anomaly
	// Ecliptic longitude: the mean longitude corrected for the Earth's
	// elliptical orbit.
	ecl := mean + rad(1.915)*math.Sin(anom) + rad(0.020)*math.Sin(2*anom)
	obl := rad(23.439 - 0.0000004*n) // obliquity of the ecliptic

	// Where the sun is on the celestial sphere.
	ra := math.Atan2(math.Cos(obl)*math.Sin(ecl), math.Cos(ecl))
	dec := math.Asin(math.Sin(obl) * math.Sin(ecl))

	// And where the observer is under it. Greenwich sidereal time turns the
	// date into a rotation of the Earth; the longitude carries it round to
	// the meridian the place stands on.
	gmst := 18.697374558 + 24.06570982441908*n
	hour := rad(math.Mod(gmst, 24)*15+lon) - ra

	φ := rad(lat)
	return deg(math.Asin(math.Sin(φ)*math.Sin(dec) +
		math.Cos(φ)*math.Cos(dec)*math.Cos(hour)))
}

func rad(d float64) float64 { return d * math.Pi / 180 }
func deg(r float64) float64 { return r * 180 / math.Pi }

package fmi

import "math"

// FeelsLike is FMI's apparent temperature — the "tuntuu kuin" reading on
// ilmatieteenlaitos.fi. It is the air temperature with a wind chill and a
// humidity correction added to it, following the SmartMet formula the
// institute uses itself. Solar radiation refines it further, but the forecast
// collection this app reads never carries it, so that term is left out.
func FeelsLike(temp, wind, hum Val) Val {
	if !temp.OK {
		return Val{}
	}
	t := temp.V
	w := math.Max(0, wind.Or(0))

	// Wind chill. The Canadian formula works in km/h; these coefficients are
	// fitted to m/s, and t0 is the temperature at which wind stops biting.
	const a, t0 = 15.0, 37.0
	chill := a + (1-a/t0)*t + a/t0*math.Pow(w+1, 0.16)*(t-t0)

	heat := t
	if hum.OK {
		heat = summerSimmer(t, hum.V)
	}
	// Both corrections apply at once: only one of them is ever far from zero.
	return Val{V: t + (chill - t) + (heat - t), OK: true}
}

// summerSimmer is the humidity half of the index. Below 14.5 °C the chart it
// comes from is undefined, and damp air has no warming effect worth naming.
func summerSimmer(t, rh float64) float64 {
	if t < 14.5 {
		return t
	}
	const ref = 0.5 // the humidity that feels like nothing at all
	r := rh / 100
	return (1.8*t - 0.55*(1-r)*(1.8*t-26) - 0.55*(1-ref)*26) / (1.8 * (1 - 0.55*(1-ref)))
}

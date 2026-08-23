package fmi

import "math"

// FeelsLike is FMI's apparent temperature ("tuntuu kuin"): air temperature
// plus wind chill and a humidity correction, following FMI's SmartMet formula.
// Its solar radiation term is dropped; this collection never carries it.
func FeelsLike(temp, wind, hum Val) Val {
	if !temp.OK {
		return Val{}
	}
	t := temp.V
	w := math.Max(0, wind.Or(0))

	// Canadian wind chill, coefficients fitted to m/s; t0 is where wind stops
	// biting.
	const a, t0 = 15.0, 37.0
	chill := a + (1-a/t0)*t + a/t0*math.Pow(w+1, 0.16)*(t-t0)

	heat := t
	if hum.OK {
		heat = summerSimmer(t, hum.V)
	}
	// Both corrections apply at once; only one is ever far from zero.
	return Val{V: t + (chill - t) + (heat - t), OK: true}
}

// summerSimmer is the humidity half of the index; it is undefined below 14.5 °C.
func summerSimmer(t, rh float64) float64 {
	if t < 14.5 {
		return t
	}
	const ref = 0.5 // humidity that feels like nothing at all
	r := rh / 100
	return (1.8*t - 0.55*(1-r)*(1.8*t-26) - 0.55*(1-ref)*26) / (1.8 * (1 - 0.55*(1-ref)))
}

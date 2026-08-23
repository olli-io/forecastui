package fmi

import (
	"math"
	"testing"
)

func TestFeelsLikeMatchesTheReferenceConditions(t *testing.T) {
	v := func(f float64) Val { return Val{V: f, OK: true} }

	cases := []struct {
		name            string
		temp, wind, hum float64
		want            float64
	}{
		// Calm air at the index's own reference humidity feels like itself.
		{"still and comfortable", 20, 0, 50, 20},
		{"wind bites in the cold", -10, 8, 85, -18.0},
		{"damp heat adds up", 28, 1, 80, 30.7},
	}
	for _, c := range cases {
		got := FeelsLike(v(c.temp), v(c.wind), v(c.hum))
		if !got.OK {
			t.Fatalf("%s: no value", c.name)
		}
		if math.Abs(got.V-c.want) > 0.1 {
			t.Errorf("%s: %.2f °C, want %.1f", c.name, got.V, c.want)
		}
	}
}

func TestFeelsLikeNeedsATemperature(t *testing.T) {
	if got := FeelsLike(Val{}, Val{V: 5, OK: true}, Val{V: 80, OK: true}); got.OK {
		t.Errorf("without a temperature there is nothing to correct, got %v", got)
	}
}

func TestFeelsLikeFallsBackToWindChillWithoutHumidity(t *testing.T) {
	cold := FeelsLike(Val{V: -5, OK: true}, Val{V: 10, OK: true}, Val{})
	if !cold.OK || cold.V >= -5 {
		t.Errorf("wind should still bite without a humidity reading, got %v", cold)
	}
}

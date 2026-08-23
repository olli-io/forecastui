package geo

import "testing"

func TestLabelNamesAPlaceAndItsContext(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   props
		want string
	}{
		{"a town takes its region",
			props{Name: "Kuopio", State: "Pohjois-Savo", Country: "Suomi"},
			"Kuopio, Pohjois-Savo, Suomi"},
		{"something inside a town takes the town",
			props{Name: "Kuopion lentoasema", City: "Siilinjärvi",
				State: "Pohjois-Savo", Country: "Suomi"},
			"Kuopion lentoasema, Siilinjärvi, Suomi"},
		{"the county stands in when there is no city",
			props{Name: "Kiruna", County: "Norrbotten", Country: "Sverige"},
			"Kiruna, Norrbotten, Sverige"},
		{"context that repeats the name is not repeated",
			props{Name: "Tromsø", City: "Tromsø", State: "Troms", Country: "Norge"},
			"Tromsø, Troms, Norge"},
		{"an address falls back to its street",
			props{HouseNumber: "17", Street: "Vuorikatu", City: "Kuopio", Country: "Suomi"},
			"17 Vuorikatu, Kuopio, Suomi"},
		{"a hit with no name at all is dropped", props{Country: "Suomi"}, ""},
	} {
		if got := label(tc.in); got != tc.want {
			t.Errorf("%s:\n got %q\nwant %q", tc.name, got, tc.want)
		}
	}
}

// Photon returns the town, the parish and the station all under one name.
func TestCollectDropsRowsThatWouldReadTheSame(t *testing.T) {
	at := func(lon, lat float64, p props) feature {
		var f feature
		f.Geometry.Coordinates = []float64{lon, lat}
		f.Properties = p
		return f
	}
	kuopio := props{Name: "Kuopio", State: "Pohjois-Savo", Country: "Suomi"}
	airport := props{Name: "Kuopion lentoasema", City: "Siilinjärvi",
		State: "Pohjois-Savo", Country: "Suomi"}

	got := collect([]feature{
		at(27.68, 62.89, kuopio),
		at(27.69, 62.90, kuopio), // the parish, same label
		at(27.79, 63.00, airport),
	}, 8)
	if len(got) != 2 {
		t.Fatalf("kept %d places, want 2: %+v", len(got), got)
	}
	if got[0].Name != "Kuopio, Pohjois-Savo, Suomi" ||
		got[1].Name != "Kuopion lentoasema, Siilinjärvi, Suomi" {
		t.Errorf("kept the wrong two: %+v", got)
	}
	// The first hit of a name wins: Photon ranks the best one first.
	if got[0].Lon != 27.68 || got[0].Lat != 62.89 {
		t.Errorf("kept the later duplicate: %+v", got[0])
	}
}

func TestCollectStopsAtTheLimitAndSkipsPointlessHits(t *testing.T) {
	var noPoint feature
	noPoint.Properties = props{Name: "Nowhere", Country: "Suomi"}
	var unnamed feature
	unnamed.Geometry.Coordinates = []float64{27, 62}

	named := func(n string) feature {
		var f feature
		f.Geometry.Coordinates = []float64{27, 62}
		f.Properties = props{Name: n, Country: "Suomi"}
		return f
	}
	got := collect([]feature{noPoint, unnamed, named("A"), named("B"), named("C")}, 2)
	if len(got) != 2 || got[0].Name != "A, Suomi" || got[1].Name != "B, Suomi" {
		t.Errorf("collect ignored the limit or kept a hit with no point: %+v", got)
	}
}

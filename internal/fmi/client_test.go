package fmi

import (
	"os"
	"testing"
	"time"
)

func loadFixture(t *testing.T) []Hour {
	t.Helper()
	body, err := os.ReadFile("testdata/turku48.json")
	if err != nil {
		t.Fatal(err)
	}
	hours, err := Parse(body)
	if err != nil {
		t.Fatal(err)
	}
	return hours
}

func TestParseMergesFeaturesIntoHours(t *testing.T) {
	hours := loadFixture(t)
	if len(hours) != 48 {
		t.Fatalf("got %d hours, want 48", len(hours))
	}
	// Every parameter lives in its own feature; a correct merge gives one hour
	// all of them.
	h := hours[0]
	for name, v := range map[string]Val{
		"temp": h.Temp, "wind": h.Wind, "gust": h.Gust,
		"dir": h.Dir, "rain": h.Rain, "pop": h.POP, "cloud": h.Cloud,
	} {
		if !v.OK {
			t.Errorf("hour 0: %s missing after merge", name)
		}
	}
	if h.Sym == 0 {
		t.Error("hour 0: weathersymbol3 missing after merge")
	}
}

func TestParseOrdersHoursAndSpacesThemHourly(t *testing.T) {
	hours := loadFixture(t)
	for i := 1; i < len(hours); i++ {
		gap := hours[i].Time.Sub(hours[i-1].Time)
		if gap != time.Hour {
			t.Fatalf("hour %d: gap %v, want 1h (ordering or merge is wrong)", i, gap)
		}
	}
	if hours[0].Time.Location() != time.UTC {
		t.Errorf("timestamps should be UTC, got %v", hours[0].Time.Location())
	}
}

func TestParseKeepsNullDistinctFromZero(t *testing.T) {
	body := []byte(`{"features":[
	  {"properties":{"time":["2026-08-22T12:00:00Z","2026-08-22T13:00:00Z"],
	                 "temperature":[null,0]}}]}`)
	hours, err := Parse(body)
	if err != nil {
		t.Fatal(err)
	}
	if hours[0].Temp.OK {
		t.Error("null temperature should be absent, not 0")
	}
	if !hours[1].Temp.OK || hours[1].Temp.V != 0 {
		t.Errorf("0 temperature should be present: %+v", hours[1].Temp)
	}
}

func TestParseAbsentParameterIsNotZero(t *testing.T) {
	body := []byte(`{"features":[
	  {"properties":{"time":["2026-08-22T12:00:00Z"],"temperature":[5]}}]}`)
	hours, err := Parse(body)
	if err != nil {
		t.Fatal(err)
	}
	if hours[0].Wind.OK {
		t.Error("wind was never returned; it must not read as 0 m/s")
	}
	if hours[0].Wind.Or(-1) != -1 {
		t.Error("Or should fall back for absent values")
	}
}

func TestParseEmptyIsErrNoData(t *testing.T) {
	if _, err := Parse([]byte(`{"features":[]}`)); err != ErrNoData {
		t.Fatalf("got %v, want ErrNoData", err)
	}
}

func TestParseSurfacesUpstreamError(t *testing.T) {
	body := []byte(`{"title":"An error occurred","detail":"Invalid locationId"}`)
	_, err := Parse(body)
	if err == nil || err == ErrNoData {
		t.Fatalf("got %v, want the upstream detail", err)
	}
}

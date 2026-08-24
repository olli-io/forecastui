package geo

import (
	"os"
	"path/filepath"
	"testing"
)

func tempConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	return dir
}

func TestLoadMissingConfigIsEmptyNotAnError(t *testing.T) {
	tempConfig(t)
	c, err := Load("testapp")
	if err != nil {
		t.Fatalf("a first run must not error: %v", err)
	}
	if c.Default != nil || len(c.Favourites) != 0 {
		t.Errorf("expected an empty config, got %+v", c)
	}
}

func TestSaveThenLoadRoundTrips(t *testing.T) {
	tempConfig(t)
	turku := Place{Name: "Turku", Lat: 60.4518, Lon: 22.2666}
	in := &Config{Default: &turku, Favourites: []Place{turku}}
	if err := in.Save("testapp"); err != nil {
		t.Fatal(err)
	}
	out, err := Load("testapp")
	if err != nil {
		t.Fatal(err)
	}
	if out.Default == nil || out.Default.Name != "Turku" {
		t.Errorf("default did not round trip: %+v", out.Default)
	}
	if !out.IsFavourite(turku) {
		t.Error("favourite did not round trip")
	}
}

func TestToggleAddsThenRemoves(t *testing.T) {
	c := &Config{}
	p := Place{Name: "Turku", Lat: 60.4518, Lon: 22.2666}
	if !c.Toggle(p) || len(c.Favourites) != 1 {
		t.Fatal("first toggle should add")
	}
	// The same point arriving under a different name is still the same place.
	if c.Toggle(Place{Name: "Åbo", Lat: 60.4518, Lon: 22.2666}) {
		t.Fatal("second toggle should remove")
	}
	if len(c.Favourites) != 0 {
		t.Errorf("favourites should be empty, got %+v", c.Favourites)
	}
}

func TestLabelFallsBackToCoordinates(t *testing.T) {
	p := Place{Lat: 60.4518, Lon: 22.2666}
	if got := p.Label(); got != "60.4518, 22.2666" {
		t.Errorf("unnamed place label: %q", got)
	}
	if got := (Place{Name: "Turku"}).Label(); got != "Turku" {
		t.Errorf("named place label: %q", got)
	}
}

func TestThemeSurvivesASave(t *testing.T) {
	tempConfig(t)
	turku := Place{Name: "Turku", Lat: 60.4518, Lon: 22.2666}
	in := &Config{Theme: "gruvbox-material", Default: &turku}
	if err := in.Save("testapp"); err != nil {
		t.Fatal(err)
	}
	// A favourite toggled later rewrites the whole file; the theme must stay.
	out, err := Load("testapp")
	if err != nil {
		t.Fatal(err)
	}
	out.Toggle(turku)
	if err := out.Save("testapp"); err != nil {
		t.Fatal(err)
	}
	again, err := Load("testapp")
	if err != nil {
		t.Fatal(err)
	}
	if again.Theme != "gruvbox-material" {
		t.Errorf("theme is %q after a toggle, want gruvbox-material", again.Theme)
	}
}

func TestOldJSONConfigIsReadThenReplaced(t *testing.T) {
	dir := tempConfig(t)
	json := filepath.Join(dir, "testapp", "config.json")
	if err := os.MkdirAll(filepath.Dir(json), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"default":{"name":"Turku","lat":60.4518,"lon":22.2666},
	          "favourites":[{"name":"Oulu","lat":65.0,"lon":25.5}]}`
	if err := os.WriteFile(json, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	c, err := Load("testapp")
	if err != nil {
		t.Fatal(err)
	}
	if c.Default == nil || c.Default.Name != "Turku" || len(c.Favourites) != 1 {
		t.Fatalf("the old config did not carry over: %+v", c)
	}

	if _, err := os.Stat(json); !os.IsNotExist(err) {
		t.Error("the old config should be gone once its contents are in the new one")
	}
	out, err := Load("testapp")
	if err != nil || out.Default == nil || out.Default.Name != "Turku" {
		t.Fatalf("round trip after migration: %+v %v", out, err)
	}
}

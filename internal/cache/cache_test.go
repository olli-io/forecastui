package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/olli-io/forecastui/internal/fmi"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	want := []fmi.Hour{{
		Time: time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC),
		Temp: fmi.Val{V: 16.5, OK: true},
		Sym:  3,
	}}
	if err := Save("testapp", 60.4518, 22.2666, 48, want); err != nil {
		t.Fatal(err)
	}
	got, at, err := Load("testapp", 60.4518, 22.2666, 48)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Temp.V != 16.5 || !got[0].Temp.OK {
		t.Errorf("round trip lost data: %+v", got)
	}
	if time.Since(at) > time.Minute {
		t.Errorf("fetched time looks wrong: %v", at)
	}
}

func TestLoadMissingEntry(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	if _, _, err := Load("testapp", 1, 2, 48); err == nil {
		t.Error("a missing cache entry should report an error the caller can ignore")
	}
}

// Each location gets its own file, so switching places does not clobber the
// forecast for the previous one.
func TestLocationsDoNotShareAFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	turku := []fmi.Hour{{Temp: fmi.Val{V: 16, OK: true}}}
	tromso := []fmi.Hour{{Temp: fmi.Val{V: 8, OK: true}}}
	if err := Save("testapp", 60.4518, 22.2666, 48, turku); err != nil {
		t.Fatal(err)
	}
	if err := Save("testapp", 69.6516, 18.9558, 48, tromso); err != nil {
		t.Fatal(err)
	}
	got, _, err := Load("testapp", 60.4518, 22.2666, 48)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Temp.V != 16 {
		t.Errorf("Turku's entry was overwritten: %+v", got[0])
	}
	entries, _ := filepath.Glob(filepath.Join(dir, "testapp", "*.json"))
	if len(entries) != 2 {
		t.Errorf("expected two cache files, got %v", entries)
	}
}

func TestStaleEntryIsDropped(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	if err := Save("testapp", 1, 2, 48, []fmi.Hour{{Temp: fmi.Val{V: 5, OK: true}}}); err != nil {
		t.Fatal(err)
	}
	// Rewrite the entry as if it had been fetched two days ago.
	p := filepath.Join(dir, "testapp", "1.0000_2.0000_48h.json")
	old := time.Now().Add(-48 * time.Hour).Format(time.RFC3339Nano)
	if err := os.WriteFile(p, []byte(`{"fetched":"`+old+`","hours":[{"Time":"2026-08-22T12:00:00Z"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	hours, _, err := Load("testapp", 1, 2, 48)
	if err != nil {
		t.Fatal(err)
	}
	if len(hours) != 0 {
		t.Errorf("a two-day-old forecast should not be shown, got %d hours", len(hours))
	}
}

// A two-day forecast cannot answer a request for the week, so the two spans
// are kept in their own files rather than one overwriting the other.
func TestSpansDoNotShareAFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	day := []fmi.Hour{{Temp: fmi.Val{V: 16, OK: true}}}
	week := []fmi.Hour{{Temp: fmi.Val{V: 8, OK: true}}}
	if err := Save("testapp", 1, 2, 48, day); err != nil {
		t.Fatal(err)
	}
	if err := Save("testapp", 1, 2, 168, week); err != nil {
		t.Fatal(err)
	}
	got, _, err := Load("testapp", 1, 2, 48)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Temp.V != 16 {
		t.Errorf("the week overwrote the two-day entry: %+v", got[0])
	}
	if _, _, err := Load("testapp", 1, 2, 24); err == nil {
		t.Error("a span that was never saved should not read back")
	}
}

func TestFresh(t *testing.T) {
	cases := []struct {
		name string
		at   time.Time
		want bool
	}{
		{"just fetched", time.Now(), true},
		{"inside the window", time.Now().Add(-Current + time.Minute), true},
		{"past the window", time.Now().Add(-Current - time.Minute), false},
		{"never fetched", time.Time{}, false},
	}
	for _, c := range cases {
		if got := Fresh(c.at); got != c.want {
			t.Errorf("%s: Fresh = %v, want %v", c.name, got, c.want)
		}
	}
}

package theme

import (
	"embed"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"

	"github.com/olli-io/forecastui/internal/render"
)

//go:embed builtin/*.toml
var builtin embed.FS

// FallbackName is the theme used when nothing chooses one.
const FallbackName = "everforest"

// DefaultName is the theme other themes are completed from: it follows the
// terminal's own colours, so a file naming one slot leaves the rest legible
// whatever the terminal is set to.
const DefaultName = "default"

// Dir returns the themes directory, honouring XDG_CONFIG_HOME through
// os.UserConfigDir.
func Dir(app string) (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, app, "themes"), nil
}

// Default is the built-in theme every other one is completed from. It is
// embedded, so a failure to parse it is a build error, not a user error.
var Default = sync.OnceValue(func() *Theme {
	body, err := builtin.ReadFile("builtin/" + DefaultName + ".toml")
	if err == nil {
		var t *Theme
		if t, err = parse(DefaultName, DefaultName+".toml", body, nil); err == nil {
			return t
		}
	}
	panic("theme: built-in default is broken: " + err.Error())
})

// Fallback is the shipped theme an install lands on. It is embedded, so like
// Default it cannot fail at runtime; a dropped-in file of the same name still
// shadows it through Load.
var Fallback = sync.OnceValue(func() *Theme {
	body, err := builtin.ReadFile("builtin/" + FallbackName + ".toml")
	if err == nil {
		var t *Theme
		if t, err = parse(FallbackName, FallbackName+".toml", body, Default()); err == nil {
			return t
		}
	}
	panic("theme: built-in fallback is broken: " + err.Error())
})

// Load resolves a theme by name, or by path when the name looks like one. An
// empty name selects FallbackName. Slots the file leaves out keep the default
// theme's value, so a two-line file is a valid override.
func Load(app, name string) (*Theme, error) {
	seed(app) // best effort: the embedded copies stay the fallback

	if name == "" {
		name = FallbackName
	}
	if isPath(name) {
		body, err := os.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("theme %s: %w", name, err)
		}
		return parse(strings.TrimSuffix(filepath.Base(name), ".toml"), name, body, Default())
	}

	// A dropped-in file wins over the embedded theme of the same name.
	if dir, err := Dir(app); err == nil {
		path := filepath.Join(dir, name+".toml")
		if body, err := os.ReadFile(path); err == nil {
			return parse(name, path, body, Default())
		}
	}
	if body, err := builtin.ReadFile("builtin/" + name + ".toml"); err == nil {
		return parse(name, name+".toml", body, Default())
	}

	return nil, fmt.Errorf("unknown theme %q; available: %s",
		name, strings.Join(Names(app), ", "))
}

// Names lists the themes on offer: the files dropped into the themes directory,
// plus the embedded ones.
func Names(app string) []string {
	seen := map[string]bool{}
	if dir, err := Dir(app); err == nil {
		if entries, err := os.ReadDir(dir); err == nil {
			for _, e := range entries {
				if n := e.Name(); !e.IsDir() && strings.HasSuffix(n, ".toml") {
					seen[strings.TrimSuffix(n, ".toml")] = true
				}
			}
		}
	}
	for _, n := range embedded() {
		seen[n] = true
	}
	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// isPath reports whether a theme was named by file rather than by name.
func isPath(name string) bool {
	return strings.HasSuffix(name, ".toml") ||
		strings.ContainsRune(name, '/') ||
		strings.ContainsRune(name, filepath.Separator)
}

// embedded lists the theme names shipped in the binary.
func embedded() []string {
	entries, err := builtin.ReadDir("builtin")
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, strings.TrimSuffix(e.Name(), ".toml"))
	}
	return out
}

// seed writes out any shipped theme that is not on disk, so an install has the
// whole set to read and copy and an upgrade delivers the ones it added. A file
// already there is left alone, edits and all.
func seed(app string) {
	dir, err := Dir(app)
	if err != nil {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	for _, name := range embedded() {
		path := filepath.Join(dir, name+".toml")
		if _, err := os.Stat(path); err == nil {
			continue
		}
		body, err := builtin.ReadFile("builtin/" + name + ".toml")
		if err != nil {
			continue
		}
		_ = os.WriteFile(path, body, 0o644)
	}
}

// parse decodes a theme file over a base. A nil base means every slot must be
// named, which only the embedded default has to satisfy.
func parse(name, path string, body []byte, base *Theme) (*Theme, error) {
	var raw map[string]string
	if _, err := toml.Decode(string(body), &raw); err != nil {
		return nil, fmt.Errorf("theme %s: %w", path, err)
	}

	out := &Theme{Name: name, Colours: make(map[render.Colour]color.Color, len(slots))}
	if base != nil {
		out = base.clone(name)
	}

	for key, value := range raw {
		slot, ok := slots[strings.ToLower(key)]
		if !ok {
			return nil, fmt.Errorf("theme %s: unknown key %q; want one of %s",
				path, key, strings.Join(slotNames(), ", "))
		}
		c, err := ParseColour(value)
		if err != nil {
			return nil, fmt.Errorf("theme %s: %s: %w", path, key, err)
		}
		out.Colours[slot] = c
	}

	for key, slot := range slots {
		if _, ok := out.Colours[slot]; !ok {
			return nil, fmt.Errorf("theme %s: missing colour %q", path, key)
		}
	}
	return out, nil
}

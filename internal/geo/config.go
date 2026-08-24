package geo

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config is the on-disk state.
type Config struct {
	// Theme names a file in the themes directory. Empty means the default.
	Theme      string  `toml:"theme,omitempty"`
	Default    *Place  `toml:"default,omitempty"`
	Favourites []Place `toml:"favourites,omitempty"`
}

// ConfigPath returns the config file location, honouring XDG_CONFIG_HOME.
func ConfigPath(app string) (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, app, "config.toml"), nil
}

// legacyPath is where the config lived before it moved to TOML.
func legacyPath(app string) (string, error) {
	path, err := ConfigPath(app)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(path), "config.json"), nil
}

// Load reads the config; a missing file yields an empty config, not an error.
// A config still in the old JSON form is read and migrated to TOML.
func Load(app string) (*Config, error) {
	path, err := ConfigPath(app)
	if err != nil {
		return &Config{}, err
	}
	body, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return loadLegacy(app)
	}
	if err != nil {
		return &Config{}, err
	}
	var c Config
	if err := toml.Unmarshal(body, &c); err != nil {
		return &Config{}, err
	}
	return &c, nil
}

func loadLegacy(app string) (*Config, error) {
	path, err := legacyPath(app)
	if err != nil {
		return &Config{}, err
	}
	body, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return &Config{}, err
	}
	// The old file had no theme, and the field names match bar the case.
	var c struct {
		Default    *Place  `json:"default"`
		Favourites []Place `json:"favourites"`
	}
	if err := json.Unmarshal(body, &c); err != nil {
		return &Config{}, err
	}
	out := &Config{Default: c.Default, Favourites: c.Favourites}
	// Move it over there and then, so the file the README describes is the one
	// on disk even for a user who never changes place. A failed write leaves
	// the JSON where it is and is read again next time.
	_ = out.Save(app)
	return out, nil
}

// Save writes the config, creating the directory if needed.
func (c *Config) Save(app string) error {
	path, err := ConfigPath(app)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var body bytes.Buffer
	if err := toml.NewEncoder(&body).Encode(c); err != nil {
		return err
	}
	// Write via a temporary file so an interrupted save cannot truncate the
	// existing config.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body.Bytes(), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	// The TOML file now holds everything the JSON one did.
	if old, err := legacyPath(app); err == nil {
		_ = os.Remove(old)
	}
	return nil
}

// Toggle adds or removes a favourite, reporting whether it is one afterwards.
func (c *Config) Toggle(p Place) bool {
	for i, f := range c.Favourites {
		if f.Same(p) {
			c.Favourites = append(c.Favourites[:i], c.Favourites[i+1:]...)
			return false
		}
	}
	c.Favourites = append(c.Favourites, p)
	return true
}

// IsFavourite reports whether a place is saved.
func (c *Config) IsFavourite(p Place) bool {
	for _, f := range c.Favourites {
		if f.Same(p) {
			return true
		}
	}
	return false
}

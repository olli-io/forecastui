package geo

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config is the on-disk state.
type Config struct {
	Default    *Place  `json:"default,omitempty"`
	Favourites []Place `json:"favourites,omitempty"`
}

// ConfigPath returns the config file location, honouring XDG_CONFIG_HOME.
func ConfigPath(app string) (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, app, "config.json"), nil
}

// Load reads the config; a missing file yields an empty config, not an error.
func Load(app string) (*Config, error) {
	path, err := ConfigPath(app)
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
	var c Config
	if err := json.Unmarshal(body, &c); err != nil {
		return &Config{}, err
	}
	return &c, nil
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
	body, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	// Write via a temporary file so an interrupted save cannot truncate the
	// existing config.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(body, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
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

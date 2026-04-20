// Package config persists the small amount of user-level state that
// yoink-n-yeet carries between invocations: which channel the install came
// from, whether background auto-updates are enabled, preview width, etc.
//
// Config is intentionally a plain JSON file — TOML was considered and
// rejected because it would add a third-party dependency for no user-visible
// benefit. Humans can still read and hand-edit the file.
package config

import (
	"encoding/json"
	"errors"
	"os"
)

// Config is the persisted configuration. Fields are stable wire-format;
// bump the Version and migrate if you change semantics.
type Config struct {
	// Version of the config schema. 1 for v0.1.0.
	Version int `json:"version"`

	// Channel is one of "stable", "nightly", "local".
	Channel string `json:"channel"`

	// AutoUpdate, when true, causes each run to kick off a background
	// update attempt. Default false — users opt in.
	AutoUpdate bool `json:"auto_update"`

	// UpdateCheck, when true, causes each run to refresh the latest-version
	// cache in the background and surface a banner if an update is
	// available. Default true.
	UpdateCheck bool `json:"update_check"`

	// PreviewWidth is the max column width for --list entry previews.
	PreviewWidth int `json:"preview_width"`

	// MaxDepth is an optional soft cap on the stack; 0 = unlimited.
	MaxDepth int `json:"max_depth"`

	// LocalRepoPath is only set when Channel == "local" and records the
	// working tree this binary was installed from.
	LocalRepoPath string `json:"local_repo_path,omitempty"`

	// InstalledAt is an RFC3339 timestamp of when the current binary was
	// installed.
	InstalledAt string `json:"installed_at,omitempty"`

	// InstalledVersion is the version that was installed (may differ from
	// the running binary if the user downgraded manually).
	InstalledVersion string `json:"installed_version,omitempty"`
}

// Default returns a Config with sane defaults. Auto-update is off; update
// checks are on.
func Default() *Config {
	return &Config{
		Version:      1,
		Channel:      "stable",
		AutoUpdate:   false,
		UpdateCheck:  true,
		PreviewWidth: 80,
		MaxDepth:     0,
	}
}

// Load reads a Config from path. If the file does not exist, Default() is
// returned without error.
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Default(), nil
		}
		return nil, err
	}
	c := Default()
	if err := json.Unmarshal(raw, c); err != nil {
		return nil, err
	}
	if c.Version == 0 {
		c.Version = 1
	}
	return c, nil
}

// Save writes c to path atomically.
func Save(path string, c *Config) error {
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

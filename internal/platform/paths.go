// Package platform resolves per-user data, config, and cache directories
// appropriate to the host OS. It is intentionally tiny and stdlib-only so
// the binary can remain dependency-free.
package platform

import (
	"os"
	"path/filepath"
	"runtime"
)

const AppName = "yoink-n-yeet"

// Paths bundles the three root directories this program cares about.
type Paths struct {
	Data   string // stack entries + stats live here
	Config string // config.json lives here
	Cache  string // update-check cache lives here
}

// Resolve picks appropriate roots for the current OS and ensures they
// exist. It prefers XDG environment variables on Linux, Apple's standard
// library locations on macOS, and %LOCALAPPDATA% on Windows.
func Resolve() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, err
	}
	var p Paths
	switch runtime.GOOS {
	case "darwin":
		p = Paths{
			Data:   filepath.Join(home, "Library", "Application Support", AppName),
			Config: filepath.Join(home, "Library", "Preferences", AppName),
			Cache:  filepath.Join(home, "Library", "Caches", AppName),
		}
	case "windows":
		local := os.Getenv("LOCALAPPDATA")
		if local == "" {
			local = filepath.Join(home, "AppData", "Local")
		}
		p = Paths{
			Data:   filepath.Join(local, AppName, "data"),
			Config: filepath.Join(local, AppName, "config"),
			Cache:  filepath.Join(local, AppName, "cache"),
		}
	default: // linux, freebsd, etc.
		data := os.Getenv("XDG_DATA_HOME")
		if data == "" {
			data = filepath.Join(home, ".local", "share")
		}
		cfg := os.Getenv("XDG_CONFIG_HOME")
		if cfg == "" {
			cfg = filepath.Join(home, ".config")
		}
		cache := os.Getenv("XDG_CACHE_HOME")
		if cache == "" {
			cache = filepath.Join(home, ".cache")
		}
		p = Paths{
			Data:   filepath.Join(data, AppName),
			Config: filepath.Join(cfg, AppName),
			Cache:  filepath.Join(cache, AppName),
		}
	}
	for _, d := range []string{p.Data, p.Config, p.Cache} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return p, err
		}
	}
	return p, nil
}

// StackDir returns the directory that holds stack entries.
func (p Paths) StackDir() string { return filepath.Join(p.Data, "stack") }

// ConfigFile returns the path to config.json.
func (p Paths) ConfigFile() string { return filepath.Join(p.Config, "config.json") }

// StatsFile returns the path to the append-only stats.jsonl file.
func (p Paths) StatsFile() string { return filepath.Join(p.Data, "stats.jsonl") }

// UpdateCacheFile returns the path to the update-check cache.
func (p Paths) UpdateCacheFile() string { return filepath.Join(p.Cache, "latest_version.json") }

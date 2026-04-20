// Package update handles two related concerns:
//
//  1. An asynchronous background check that compares the running version
//     to the latest GitHub release, caches the result with a 24h TTL, and
//     produces an unobtrusive stderr banner when an update is available.
//  2. Self-update of the binary: download, sha256-verify, atomically
//     swap, and preserve symlinks.
//
// All network I/O is bounded by short timeouts. Failures are silent — a
// broken network must never break a user's copy/paste.
package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/CoreyRDean/yoink-n-yeet/internal/buildinfo"
	"github.com/CoreyRDean/yoink-n-yeet/internal/config"
)

// Repo is the upstream GitHub coordinate for release lookups.
const Repo = "CoreyRDean/yoink-n-yeet"

// cache is the shape written to the update-check cache file.
type cache struct {
	CheckedAt time.Time `json:"checked_at"`
	Latest    string    `json:"latest"`
	Channel   string    `json:"channel"`
}

// Release is a trimmed GitHub Release payload.
type Release struct {
	TagName    string  `json:"tag_name"`
	Prerelease bool    `json:"prerelease"`
	Assets     []Asset `json:"assets"`
	HTMLURL    string  `json:"html_url"`
}

// Asset is a trimmed GitHub release asset.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// BackgroundCheck spawns a detached goroutine that refreshes the latest
// version for the user's channel. The call returns immediately.
//
// Two things are deliberately asymmetric here:
//   - The check itself happens in the background, so a slow or offline
//     network never blocks the user's invocation.
//   - The banner (if any) is written synchronously at program start
//     *from the previous run's cached result*, so users see it on the
//     very next invocation but never pay latency for it.
func BackgroundCheck(cfg *config.Config, cachePath string) {
	if cfg == nil || !cfg.UpdateCheck {
		return
	}
	go func() {
		defer func() {
			_ = recover() // never let a background goroutine crash the program
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()
		rel, err := fetchLatest(ctx, cfg.Channel)
		if err != nil || rel == nil {
			return
		}
		c := cache{CheckedAt: time.Now().UTC(), Latest: rel.TagName, Channel: cfg.Channel}
		raw, err := json.Marshal(c)
		if err != nil {
			return
		}
		_ = os.MkdirAll(filepath.Dir(cachePath), 0o755)
		_ = os.WriteFile(cachePath, raw, 0o600)
	}()
}

// MaybeBanner writes a single-line stderr banner if the cached latest
// version is newer than the running binary. It's best-effort and silent on
// any error.
func MaybeBanner(w io.Writer, cfg *config.Config, cachePath string) {
	if cfg == nil || !cfg.UpdateCheck {
		return
	}
	raw, err := os.ReadFile(cachePath)
	if err != nil {
		return
	}
	var c cache
	if err := json.Unmarshal(raw, &c); err != nil {
		return
	}
	if c.Channel != cfg.Channel {
		return
	}
	if c.Latest == "" || c.Latest == buildinfo.Version {
		return
	}
	// Older local dev builds (v0.0.0-dev) should also nudge users.
	if !isNewer(c.Latest, buildinfo.Version) {
		return
	}
	fmt.Fprintf(w, "\033[2m[yoink-n-yeet] update available: %s → %s  (run 'yt --update')\033[0m\n",
		buildinfo.Version, c.Latest)
}

// FetchLatest returns the most recent release for the given channel
// ("stable" or "nightly"). Context-scoped; respects caller timeouts.
func FetchLatest(ctx context.Context, channel string) (*Release, error) {
	return fetchLatest(ctx, channel)
}

func fetchLatest(ctx context.Context, channel string) (*Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases", Repo)
	if channel == "stable" {
		url = fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", Repo)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "yoink-n-yeet/"+buildinfo.Version)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("github: %s", resp.Status)
	}
	if channel == "stable" {
		var r Release
		if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
			return nil, err
		}
		return &r, nil
	}
	// Nightly: scan releases for the most recent prerelease tagged -nightly.
	var rs []Release
	if err := json.NewDecoder(resp.Body).Decode(&rs); err != nil {
		return nil, err
	}
	for _, r := range rs {
		if r.Prerelease && strings.Contains(r.TagName, "-nightly.") {
			return &r, nil
		}
	}
	return nil, errors.New("no nightly release found")
}

// isNewer reports whether latest is strictly newer than current. It uses
// a naive lexicographic comparison that works for well-formed semver but
// is intentionally conservative — the remote is treated as the source of
// truth, so false positives just mean "refresh your tools."
func isNewer(latest, current string) bool {
	if current == "v0.0.0-dev" || current == "unknown" || current == "" {
		return true
	}
	return latest != current && latest > current
}

// AssetFor returns the asset in rel that matches the current host
// (GOOS/GOARCH) naming convention used by goreleaser:
// yoink-n-yeet_<version>_<os>_<arch>.tar.gz
// or .zip on Windows.
func AssetFor(rel *Release) *Asset {
	goos, goarch := runtime.GOOS, runtime.GOARCH
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}
	needle := fmt.Sprintf("_%s_%s%s", goos, goarch, ext)
	for i := range rel.Assets {
		if strings.HasSuffix(rel.Assets[i].Name, needle) {
			return &rel.Assets[i]
		}
	}
	return nil
}

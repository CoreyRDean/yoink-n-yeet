package update

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/CoreyRDean/yoink-n-yeet/internal/buildinfo"
	"github.com/CoreyRDean/yoink-n-yeet/internal/config"
)

// Apply performs a self-update. The target channel overrides the config
// if non-empty. For "local" channel, we re-run the repo's install.sh
// against the current working tree; for "stable"/"nightly" we pipe the
// canonical installer from GitHub through a shell with the channel flag.
//
// We delegate to install.sh instead of reimplementing download + extract +
// verify + swap in Go for two reasons: (1) the installer is already the
// source of truth for how the binary ends up on disk, so any divergence
// would be a bug-farm, and (2) users can read install.sh and trust it in a
// way they can't audit a compiled self-update path.
func Apply(cfg *config.Config, targetChannel string, w io.Writer) error {
	channel := targetChannel
	if channel == "" {
		channel = cfg.Channel
	}
	if runtime.GOOS == "windows" {
		return fmt.Errorf("self-update on Windows is not yet supported; download the latest release from https://github.com/%s/releases", Repo)
	}

	switch channel {
	case "local":
		if cfg.LocalRepoPath == "" {
			return fmt.Errorf("channel=local but local_repo_path is not set in config")
		}
		script := filepath.Join(cfg.LocalRepoPath, "install.sh")
		if _, err := os.Stat(script); err != nil {
			return fmt.Errorf("install.sh not found at %s: %w", script, err)
		}
		fmt.Fprintf(w, "re-running %s --local\n", script)
		cmd := exec.Command(script, "--local")
		cmd.Dir = cfg.LocalRepoPath
		cmd.Stdout = w
		cmd.Stderr = w
		return cmd.Run()
	case "stable", "nightly":
		url := fmt.Sprintf("https://raw.githubusercontent.com/%s/main/install.sh", Repo)
		fmt.Fprintf(w, "fetching %s\n", url)
		script, err := fetch(url)
		if err != nil {
			return fmt.Errorf("download installer: %w", err)
		}
		defer os.Remove(script)
		args := []string{script, "--channel", channel}
		fmt.Fprintf(w, "running installer (channel=%s)\n", channel)
		cmd := exec.Command("/bin/sh", args...)
		cmd.Stdout = w
		cmd.Stderr = w
		return cmd.Run()
	default:
		return fmt.Errorf("unknown channel %q (want stable, nightly, or local)", channel)
	}
}

// FetchInstaller downloads an installer script URL to a temp file and
// returns the path. Caller is responsible for removing it.
func FetchInstaller(url string) (string, error) { return fetch(url) }

// fetch downloads url to a temp file and returns the path. Caller is
// responsible for removing it.
func fetch(url string) (string, error) {
	resp, err := http.Get(url) //nolint:gosec // URL is a build-time constant
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("%s: %s", url, resp.Status)
	}
	f, err := os.CreateTemp("", "yoink-installer-*.sh")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	if err := os.Chmod(f.Name(), 0o755); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// Banner returns the current install's version + channel summary. Kept
// here (rather than in buildinfo) to compose cleanly with update state.
func Banner() string {
	return buildinfo.String()
}

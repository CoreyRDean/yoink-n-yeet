// Package buildinfo carries version and provenance metadata that is
// injected by the build system via -ldflags. These variables intentionally
// use string types so they can be overridden with `-X`.
package buildinfo

import (
	"fmt"
	"os/exec"
	"strings"
)

// These are overridden at build time with:
//
//	go build -ldflags "-X <module>/internal/buildinfo.Version=v0.1.0 ..."
var (
	// Version is the semver tag this binary was built from (e.g. "v0.1.0"),
	// or "v0.0.0-dev" for unbranded local builds.
	Version = "v0.0.0-dev"
	// Commit is the git SHA of HEAD at build time.
	Commit = "unknown"
	// Channel is one of "stable", "nightly", or "local".
	Channel = "stable"
	// Date is the UTC build timestamp in RFC3339 format.
	Date = "unknown"
	// RepoPath is only populated for `channel=local` installs and records
	// the absolute path of the working tree the binary was built from.
	RepoPath = ""
)

// IsDirty reports whether RepoPath has uncommitted changes. It is only
// meaningful when Channel == "local" and RepoPath is set. Errors are
// swallowed; callers get false on any failure.
func IsDirty() bool {
	if RepoPath == "" {
		return false
	}
	cmd := exec.Command("git", "-C", RepoPath, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}

// String renders a human-readable --version line for the resolved build.
func String() string {
	base := fmt.Sprintf("yoink-n-yeet %s (channel=%s, commit=%s, built=%s)",
		Version, Channel, shortCommit(Commit), Date)
	if Channel == "local" && RepoPath != "" {
		dirty := ""
		if IsDirty() {
			dirty = "+dirty"
		}
		base += fmt.Sprintf("\n  repo: %s\n  commit%s: %s", RepoPath, dirty, Commit)
	}
	return base
}

func shortCommit(c string) string {
	if len(c) >= 12 {
		return c[:12]
	}
	return c
}

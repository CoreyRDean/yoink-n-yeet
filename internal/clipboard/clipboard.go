// Package clipboard abstracts the host OS clipboard behind a small interface
// and picks a concrete backend based on runtime detection.
//
// All backends shell out to standard system utilities. This keeps the binary
// dependency-free and avoids linking against platform-specific C libraries.
// The trade-off is that users must have the relevant tool installed
// (xclip/xsel on X11, wl-copy on Wayland). `--doctor` surfaces gaps.
package clipboard

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
)

// Clipboard is the minimal surface yoink-n-yeet needs from the OS clipboard.
type Clipboard interface {
	Copy(data []byte) error
	Paste() ([]byte, error)
	// Backend returns a short identifier like "pbcopy", "wl-copy", "xclip",
	// "xsel", or "windows". Useful for --doctor output.
	Backend() string
}

// ErrNoBackend is returned when no clipboard tool is available.
var ErrNoBackend = errors.New("no clipboard backend found")

// Detect picks the best available backend for the current host.
func Detect() (Clipboard, error) {
	switch runtime.GOOS {
	case "darwin":
		return &cmdClipboard{copyCmd: []string{"pbcopy"}, pasteCmd: []string{"pbpaste"}, name: "pbcopy"}, nil
	case "windows":
		return &cmdClipboard{
			copyCmd:  []string{"clip"},
			pasteCmd: []string{"powershell", "-NoProfile", "-Command", "Get-Clipboard -Raw"},
			name:     "windows",
		}, nil
	default:
		// Linux / BSD: prefer Wayland when we detect a session, otherwise X11.
		if os.Getenv("WAYLAND_DISPLAY") != "" && have("wl-copy") && have("wl-paste") {
			return &cmdClipboard{copyCmd: []string{"wl-copy"}, pasteCmd: []string{"wl-paste", "--no-newline"}, name: "wl-copy"}, nil
		}
		if have("xclip") {
			return &cmdClipboard{copyCmd: []string{"xclip", "-selection", "clipboard"}, pasteCmd: []string{"xclip", "-selection", "clipboard", "-o"}, name: "xclip"}, nil
		}
		if have("xsel") {
			return &cmdClipboard{copyCmd: []string{"xsel", "--clipboard", "--input"}, pasteCmd: []string{"xsel", "--clipboard", "--output"}, name: "xsel"}, nil
		}
		// Microsoft WSL: `clip.exe` is usually on PATH.
		if have("clip.exe") {
			return &cmdClipboard{copyCmd: []string{"clip.exe"}, pasteCmd: []string{"powershell.exe", "-NoProfile", "-Command", "Get-Clipboard"}, name: "wsl"}, nil
		}
		return nil, ErrNoBackend
	}
}

type cmdClipboard struct {
	copyCmd, pasteCmd []string
	name              string
}

func (c *cmdClipboard) Backend() string { return c.name }

func (c *cmdClipboard) Copy(data []byte) error {
	cmd := exec.Command(c.copyCmd[0], c.copyCmd[1:]...)
	cmd.Stdin = bytes.NewReader(data)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w: %s", c.name, err, stderr.String())
	}
	return nil
}

func (c *cmdClipboard) Paste() ([]byte, error) {
	cmd := exec.Command(c.pasteCmd[0], c.pasteCmd[1:]...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", c.name, err)
	}
	return out, nil
}

// have reports whether a command exists on PATH.
func have(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// Doctor runs diagnostics and writes a human-readable report to w. It
// returns a nil error if a backend is usable and a non-nil error otherwise,
// so callers can exit non-zero if the environment is broken.
func Doctor(w io.Writer) error {
	fmt.Fprintf(w, "os=%s arch=%s\n", runtime.GOOS, runtime.GOARCH)
	cb, err := Detect()
	if err != nil {
		fmt.Fprintln(w, "clipboard backend: none available")
		suggestInstall(w)
		return err
	}
	fmt.Fprintf(w, "clipboard backend: %s\n", cb.Backend())
	// Round-trip smoke test with a short sentinel.
	sentinel := []byte("yoink-n-yeet doctor smoke test")
	if err := cb.Copy(sentinel); err != nil {
		fmt.Fprintf(w, "copy: FAIL (%v)\n", err)
		return err
	}
	fmt.Fprintln(w, "copy: ok")
	got, err := cb.Paste()
	if err != nil {
		// Paste failure is non-fatal for our purposes — we only copy in
		// normal operation — but we still surface it.
		fmt.Fprintf(w, "paste: FAIL (%v)\n", err)
		return nil
	}
	if !bytes.Contains(got, sentinel) {
		fmt.Fprintf(w, "paste: mismatch (got %q)\n", truncate(string(got), 40))
		return nil
	}
	fmt.Fprintln(w, "paste: ok")
	return nil
}

func suggestInstall(w io.Writer) {
	switch runtime.GOOS {
	case "linux", "freebsd":
		fmt.Fprintln(w, "install one of: wl-copy (Wayland), xclip, xsel")
		fmt.Fprintln(w, "  apt:  sudo apt install wl-clipboard xclip")
		fmt.Fprintln(w, "  dnf:  sudo dnf install wl-clipboard xclip")
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

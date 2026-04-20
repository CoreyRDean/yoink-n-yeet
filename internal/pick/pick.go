// Package pick implements a tiny, dependency-free terminal chooser.
//
// It renders a list of items to stderr, places the terminal in raw mode via
// `stty`, and reads single-byte arrow-key / j-k / Enter / q / Ctrl-C events
// from stdin to drive selection. On exit — whether a selection was made or
// the user bailed — the previous `stty` state is restored and the cursor is
// re-shown.
//
// stty was chosen over golang.org/x/term to keep the binary stdlib-only.
// The trade-off is that this package only works where `stty` exists (every
// Unix, WSL; not native Windows). The CLI already guards entry to the
// picker on `isTTY(os.Stdin)`, and interactive selection on native Windows
// is a non-goal for v0.1.x — users on that platform can use `--pick N`.
package pick

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
)

// Item is a single row in the menu. Label is printed verbatim; callers are
// responsible for width-fitting and preview generation.
type Item struct {
	Label string
}

// ErrCancelled is returned when the user bailed out with q / Ctrl-C / ESC.
var ErrCancelled = errors.New("picker cancelled")

// Run displays items with an arrow cursor and returns the chosen 0-based
// index. The terminal is restored on all exit paths including SIGINT.
//
// title (when non-empty) is printed above the list.
func Run(items []Item, title string) (int, error) {
	if len(items) == 0 {
		return -1, errors.New("nothing to pick")
	}

	saved, err := saveStty()
	if err != nil {
		return -1, fmt.Errorf("stty -g: %w", err)
	}
	if err := enterRaw(); err != nil {
		_ = restoreStty(saved)
		return -1, fmt.Errorf("stty raw: %w", err)
	}

	// Make absolutely sure we put the terminal back, including on SIGINT.
	// We install our own handler so Ctrl-C exits cleanly rather than leaving
	// the tty in raw mode.
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		select {
		case <-sigc:
			_ = restoreStty(saved)
			fmt.Fprint(os.Stderr, showCursor+"\r\n")
			os.Exit(130)
		case <-done:
			signal.Stop(sigc)
		}
	}()
	defer func() {
		close(done)
		_ = restoreStty(saved)
		fmt.Fprint(os.Stderr, showCursor)
	}()

	fmt.Fprint(os.Stderr, hideCursor)
	if title != "" {
		fmt.Fprintf(os.Stderr, "%s\r\n", title)
	}

	selected := 0
	rendered := render(os.Stderr, items, selected)

	reader := bufio.NewReader(os.Stdin)
	for {
		b, err := reader.ReadByte()
		if err != nil {
			// EOF on stdin = bail. Treat as cancel.
			return -1, ErrCancelled
		}
		switch b {
		case 3, 'q', 'Q': // Ctrl-C, q
			fmt.Fprint(os.Stderr, "\r\n")
			return -1, ErrCancelled
		case '\r', '\n', ' ':
			fmt.Fprint(os.Stderr, "\r\n")
			return selected, nil
		case 'j':
			selected = clamp(selected+1, 0, len(items)-1)
		case 'k':
			selected = clamp(selected-1, 0, len(items)-1)
		case 0x1b: // ESC — possibly the start of a CSI arrow sequence
			// Peek the next two bytes. If they aren't '[A'/'[B', treat as
			// bare ESC = cancel.
			b1, err1 := reader.ReadByte()
			if err1 != nil || b1 != '[' {
				fmt.Fprint(os.Stderr, "\r\n")
				return -1, ErrCancelled
			}
			b2, err2 := reader.ReadByte()
			if err2 != nil {
				return -1, ErrCancelled
			}
			switch b2 {
			case 'A':
				selected = clamp(selected-1, 0, len(items)-1)
			case 'B':
				selected = clamp(selected+1, 0, len(items)-1)
			case 'H': // Home
				selected = 0
			case 'F': // End
				selected = len(items) - 1
			}
		}

		// Redraw in place: move cursor up by the number of lines we rendered
		// last time, clear everything from there to end of screen, re-render.
		fmt.Fprintf(os.Stderr, "\033[%dA\r\033[J", rendered)
		rendered = render(os.Stderr, items, selected)
	}
}

const (
	hideCursor = "\033[?25l"
	showCursor = "\033[?25h"
	boldOn     = "\033[1m"
	reverseOn  = "\033[7m"
	sgrReset   = "\033[0m"
)

// render writes items with the selected row highlighted and returns the
// number of visual lines emitted.
func render(w io.Writer, items []Item, selected int) int {
	for i, it := range items {
		if i == selected {
			fmt.Fprintf(w, "%s> %s%s\r\n", reverseOn, it.Label, sgrReset)
		} else {
			fmt.Fprintf(w, "  %s\r\n", it.Label)
		}
	}
	return len(items)
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// --- stty wrappers ---

func saveStty() (string, error) {
	cmd := exec.Command("stty", "-g")
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func enterRaw() error {
	// -icanon: no line buffering. -echo: don't echo typed keys.
	// isig kept on so SIGINT still fires and our handler cleans up.
	cmd := exec.Command("stty", "-icanon", "-echo", "min", "1", "time", "0")
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func restoreStty(saved string) error {
	cmd := exec.Command("stty", saved)
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// Command yoink-n-yeet is a cross-platform clipboard-stack CLI.
//
// The same binary is invoked under four names (all symlinks):
//
//	yoink  / yk   push  (run a command and push its stdout, or push stdin)
//	yeet   / yt   pop   (emit the top entry to stdout and remove it)
//
// argv[0] determines the *default* action. Every flag below works on either
// name, so e.g. `yt --list` and `yk --list` are identical.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/CoreyRDean/yoink-n-yeet/internal/buildinfo"
	"github.com/CoreyRDean/yoink-n-yeet/internal/clipboard"
	"github.com/CoreyRDean/yoink-n-yeet/internal/config"
	"github.com/CoreyRDean/yoink-n-yeet/internal/pick"
	"github.com/CoreyRDean/yoink-n-yeet/internal/platform"
	"github.com/CoreyRDean/yoink-n-yeet/internal/redact"
	"github.com/CoreyRDean/yoink-n-yeet/internal/stack"
	"github.com/CoreyRDean/yoink-n-yeet/internal/stats"
	"github.com/CoreyRDean/yoink-n-yeet/internal/update"
)

// action is the default operation picked from argv[0].
type action int

const (
	actPush action = iota
	actPop
)

func defaultAction() action {
	name := filepath.Base(os.Args[0])
	// Trim platform-specific suffix so a .exe rename on Windows still works.
	name = strings.TrimSuffix(name, ".exe")
	switch name {
	case "yeet", "yt":
		return actPop
	default:
		// yoink / yk / yoink-n-yeet / unknown → push
		return actPush
	}
}

// opts captures every flag the CLI supports. We parse argv manually rather
// than via the flag package because we need to stop consuming flags at the
// first positional arg when the action is push (so the user's command and
// args are passed through verbatim).
type opts struct {
	// mutually-exclusive operation modifiers
	list      bool
	listJSON  bool
	show      bool   // was --show seen?
	showArg   string // raw index token: "", "first", "last", or decimal
	peek      bool
	pickFlag  bool   // was --pick seen?
	pickArg   string // raw index token: "", "first", "last", or decimal
	dry       bool
	drain     bool
	drainDays int
	drainHrs  int
	statsShow bool
	statsJSON bool
	doctor    bool
	version   bool
	update    string // "", "stable", "nightly"
	stable    bool
	autoUpd   string // "", "on", "off", "status"
	uninstall bool

	report      bool
	reportTitle string
	reportBody  string

	clipboardSrc bool // --cb / -c: use the OS clipboard as the data source

	// behavioral
	noUpdateCheck bool
	help          bool

	// positional remainder after -- or first non-flag token
	rest []string
}

// parseArgs consumes os.Args[1:] and produces opts. Unknown flags are
// returned as errors.
func parseArgs(argv []string) (*opts, error) {
	o := &opts{}
	i := 0
	for i < len(argv) {
		a := argv[i]
		switch {
		case a == "--":
			o.rest = append(o.rest, argv[i+1:]...)
			return o, nil
		case a == "--list":
			o.list = true
		case a == "--json" && (o.list || o.statsShow):
			if o.list {
				o.listJSON = true
			}
			if o.statsShow {
				o.statsJSON = true
			}
		case a == "--show":
			// Optional argument: an integer, "first" (top = 0), or "last"
			// (bottom = Len-1). Missing argument defaults to "first", which
			// matches --peek.
			o.show = true
			if i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "-") {
				o.showArg = argv[i+1]
				i++
			}
		case a == "--peek":
			o.peek = true
		case a == "--pick":
			// Optional argument: integer, "first", "last", or nothing.
			// No argument + TTY = interactive picker; no argument + no TTY = no-op.
			o.pickFlag = true
			if i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "-") {
				o.pickArg = argv[i+1]
				i++
			}
		case a == "--dry":
			o.dry = true
		case a == "--drain", a == "--clear":
			o.drain = true
		case a == "--days":
			if i+1 >= len(argv) {
				return nil, errors.New("--days requires a number")
			}
			n, err := strconv.Atoi(argv[i+1])
			if err != nil || n < 0 {
				return nil, fmt.Errorf("--days: invalid value %q", argv[i+1])
			}
			o.drainDays = n
			i++
		case a == "--hours":
			if i+1 >= len(argv) {
				return nil, errors.New("--hours requires a number")
			}
			n, err := strconv.Atoi(argv[i+1])
			if err != nil || n < 0 {
				return nil, fmt.Errorf("--hours: invalid value %q", argv[i+1])
			}
			o.drainHrs = n
			i++
		case a == "--stats":
			o.statsShow = true
		case a == "--doctor":
			o.doctor = true
		case a == "--version":
			o.version = true
		case a == "--update":
			o.update = "stable"
			if i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "-") {
				o.update = argv[i+1]
				i++
			}
		case a == "--stable":
			o.stable = true
		case a == "--auto-update":
			if i+1 >= len(argv) {
				return nil, errors.New("--auto-update requires on|off|status")
			}
			o.autoUpd = argv[i+1]
			i++
		case a == "--uninstall":
			o.uninstall = true
		case a == "--report":
			o.report = true
			// Greedy-consume up to two positional trailers as title + body.
			// Users can skip either by passing nothing; we'll prompt on a
			// TTY or fail loudly otherwise.
			if i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "-") {
				o.reportTitle = argv[i+1]
				i++
				if i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "-") {
					o.reportBody = argv[i+1]
					i++
				}
			}
		case a == "--no-update-check":
			o.noUpdateCheck = true
		case a == "--cb", a == "-c":
			// Treat the OS clipboard as the source. On yoink/yk this imports
			// the current clipboard contents onto the stack; on yeet/yt this
			// reads the clipboard to stdout and *bypasses* the stack entirely
			// — handy for piping whatever's in the clipboard without popping
			// anything the user carefully stacked up.
			o.clipboardSrc = true
		case a == "-h", a == "--help":
			o.help = true
		case strings.HasPrefix(a, "-") && a != "-":
			// Unknown flag — stop flag parsing and treat the rest as the
			// user's command. This lets `yk ls -la` work without us
			// trying to interpret `-la`.
			o.rest = append(o.rest, argv[i:]...)
			return o, nil
		default:
			o.rest = append(o.rest, argv[i:]...)
			return o, nil
		}
		i++
	}
	return o, nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "yoink-n-yeet:", err)
		os.Exit(1)
	}
}

func run() error {
	o, err := parseArgs(os.Args[1:])
	if err != nil {
		return err
	}
	if o.help {
		printHelp(os.Stdout)
		return nil
	}
	if o.version {
		fmt.Println(update.Banner())
		return nil
	}

	paths, err := platform.Resolve()
	if err != nil {
		return fmt.Errorf("resolve paths: %w", err)
	}
	cfg, err := config.Load(paths.ConfigFile())
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	// Channel defaults to the one baked into the binary if config is fresh.
	if cfg.Channel == "" {
		cfg.Channel = buildinfo.Channel
	}
	if buildinfo.Channel == "local" && buildinfo.RepoPath != "" && cfg.LocalRepoPath == "" {
		cfg.LocalRepoPath = buildinfo.RepoPath
	}

	// Fire off the async update check early so it can race alongside real
	// work. Skipped if the user opted out or config disables it.
	if !o.noUpdateCheck {
		update.BackgroundCheck(cfg, paths.UpdateCacheFile())
	}
	// Show a banner from the *previous* run's cache, on stderr only so we
	// never corrupt stdout pipes.
	update.MaybeBanner(os.Stderr, cfg, paths.UpdateCacheFile())

	// Dispatch, in priority order. Single-purpose flags win over default
	// push/pop so that e.g. `yk --list cat file` treats --list as the
	// intent and ignores the positional command.
	switch {
	case o.uninstall:
		return doUninstall(cfg)
	case o.report:
		return doReport(o.reportTitle, o.reportBody)
	case o.doctor:
		return clipboard.Doctor(os.Stdout)
	case o.statsShow:
		return doStats(paths, o.statsJSON)
	case o.autoUpd != "":
		return doAutoUpdate(paths, cfg, o.autoUpd)
	case o.stable:
		return update.Apply(cfg, "stable", os.Stderr)
	case o.update != "":
		return update.Apply(cfg, o.update, os.Stderr)
	case o.list:
		return doList(paths, cfg, o.listJSON)
	case o.show:
		return doShow(paths, o.showArg)
	case o.peek:
		return doShow(paths, "first")
	case o.pickFlag:
		return doPick(paths, o.pickArg)
	case o.drain:
		return doDrain(paths, o)
	}

	// Default-action branch: push or pop. --cb/-c short-circuits both sides:
	// push imports the OS clipboard onto the stack; pop emits the OS
	// clipboard to stdout without touching the stack.
	switch defaultAction() {
	case actPush:
		if o.clipboardSrc {
			return doPushClipboard(paths, cfg, o)
		}
		return doPush(paths, cfg, o)
	case actPop:
		if o.clipboardSrc {
			return doPasteClipboardPassthrough()
		}
		return doPop(paths, cfg, o)
	}
	return nil
}

// doPushClipboard reads the OS clipboard and pushes the bytes onto the
// stack as a regular entry (source: "clipboard"). Useful for grabbing
// something you copied from another app into the yoink stack.
func doPushClipboard(paths platform.Paths, cfg *config.Config, o *opts) error {
	cb, err := clipboard.Detect()
	if err != nil {
		return fmt.Errorf("clipboard: %w", err)
	}
	data, err := cb.Paste()
	if err != nil {
		return fmt.Errorf("clipboard read: %w", err)
	}
	if len(data) == 0 {
		fmt.Fprintln(os.Stderr, "clipboard is empty; nothing to push")
		return nil
	}
	if o.dry {
		if ok, err := confirmDryPush(data, "clipboard"); err != nil || !ok {
			if err != nil {
				return err
			}
			fmt.Fprintln(os.Stderr, "trashed.")
			return nil
		}
	}
	s, err := stack.New(paths.StackDir())
	if err != nil {
		return err
	}
	return pushCommit(s, cfg, paths, data, "clipboard", o)
}

// doPasteClipboardPassthrough emits the current OS clipboard contents to
// stdout without touching the stack. It's the stack-free counterpart to
// `yt`; useful for piping clipboard contents into another command when you
// don't want to consume a stack entry.
func doPasteClipboardPassthrough() error {
	cb, err := clipboard.Detect()
	if err != nil {
		return fmt.Errorf("clipboard: %w", err)
	}
	data, err := cb.Paste()
	if err != nil {
		return fmt.Errorf("clipboard read: %w", err)
	}
	_, err = os.Stdout.Write(data)
	return err
}

// ---------- actions ----------

func doPush(paths platform.Paths, cfg *config.Config, o *opts) error {
	s, err := stack.New(paths.StackDir())
	if err != nil {
		return err
	}
	var (
		data   []byte
		source string
	)
	if len(o.rest) > 0 {
		// Run the user's command, capture stdout, stream stderr through.
		cmd := exec.Command(o.rest[0], o.rest[1:]...)
		var buf bytes.Buffer
		cmd.Stdout = &buf
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		err := cmd.Run()
		// We push whatever stdout was produced even on non-zero exit so
		// partial output isn't lost; the error surfaces afterwards.
		data = buf.Bytes()
		source = strings.Join(o.rest, " ")
		if err != nil {
			// Still push, but warn and preserve exit semantics.
			if len(data) > 0 {
				if err2 := pushCommit(s, cfg, paths, data, source, o); err2 != nil {
					return err2
				}
			}
			return err
		}
	} else if !isTTY(os.Stdin) {
		// Pipe in.
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		data = b
		source = "stdin"
	} else {
		return errors.New("nothing to push: provide a command (e.g. 'yk cat file') or pipe input (e.g. 'cmd | yk')")
	}

	if o.dry {
		if ok, err := confirmDryPush(data, source); err != nil || !ok {
			if err != nil {
				return err
			}
			fmt.Fprintln(os.Stderr, "trashed.")
			return nil
		}
	}
	return pushCommit(s, cfg, paths, data, source, o)
}

func pushCommit(s *stack.Stack, cfg *config.Config, paths platform.Paths, data []byte, source string, _ *opts) error {
	e, err := s.Push(data, source)
	if err != nil {
		return err
	}
	// Mirror the top of the stack to the OS clipboard. Failure here is
	// non-fatal — the stack still has the entry.
	if cb, err := clipboard.Detect(); err == nil {
		_ = cb.Copy(data)
	}
	_ = stats.Append(paths.StatsFile(), stats.Event{
		Time:   time.Now().UTC(),
		Op:     stats.OpPush,
		Size:   e.Size,
		Source: source,
	})
	return nil
}

func doPop(paths platform.Paths, cfg *config.Config, o *opts) error {
	s, err := stack.New(paths.StackDir())
	if err != nil {
		return err
	}
	// Dry-mode: preview top + confirm before consuming.
	if o.dry {
		e, data, err := s.Peek()
		if err != nil {
			if errors.Is(err, stack.ErrEmpty) {
				fmt.Fprintln(os.Stderr, "stack is empty")
				return nil
			}
			return err
		}
		fmt.Fprintln(os.Stderr, formatDryPopPreview(e, data, cfg.PreviewWidth))
		if ok, err := promptYN(os.Stderr, "pop this entry?", false); err != nil || !ok {
			if err != nil {
				return err
			}
			fmt.Fprintln(os.Stderr, "cancelled.")
			return nil
		}
	}

	e, data, err := s.Pop()
	if err != nil {
		if errors.Is(err, stack.ErrEmpty) {
			fmt.Fprintln(os.Stderr, "stack is empty")
			return nil
		}
		return err
	}
	// Emit payload to stdout.
	if _, err := os.Stdout.Write(data); err != nil {
		return err
	}
	// Re-mirror the new top (or clear the clipboard if empty).
	if cb, err := clipboard.Detect(); err == nil {
		if _, top, err := s.Peek(); err == nil {
			_ = cb.Copy(top)
		} else if errors.Is(err, stack.ErrEmpty) {
			_ = cb.Copy(nil)
		}
	}
	ageMs := time.Since(e.Created).Milliseconds()
	_ = stats.Append(paths.StatsFile(), stats.Event{
		Time:  time.Now().UTC(),
		Op:    stats.OpPop,
		Size:  e.Size,
		AgeMs: ageMs,
	})
	return nil
}

func doList(paths platform.Paths, cfg *config.Config, asJSON bool) error {
	s, err := stack.New(paths.StackDir())
	if err != nil {
		return err
	}
	ents, err := s.List()
	if err != nil {
		return err
	}
	if asJSON {
		return json.NewEncoder(os.Stdout).Encode(ents)
	}
	if len(ents) == 0 {
		fmt.Fprintln(os.Stderr, "stack is empty")
		return nil
	}
	for i, e := range ents {
		raw, err := os.ReadFile(e.Path)
		if err != nil {
			fmt.Fprintf(os.Stdout, "[%d] %s  %s  <unreadable: %v>\n",
				i, e.Created.Local().Format("2006-01-02 15:04:05"), humanSize(e.Size), err)
			continue
		}
		pv := redact.Preview(string(raw), cfg.PreviewWidth)
		fmt.Fprintf(os.Stdout, "[%d] %s  %s  %s\n",
			i, e.Created.Local().Format("2006-01-02 15:04:05"), humanSize(e.Size), pv)
	}
	return nil
}

func doShow(paths platform.Paths, arg string) error {
	s, err := stack.New(paths.StackDir())
	if err != nil {
		return err
	}
	n, err := s.Len()
	if err != nil {
		return err
	}
	if n == 0 {
		fmt.Fprintln(os.Stderr, "stack is empty")
		return nil
	}
	idx, err := resolveIndex(arg, n)
	if err != nil {
		return err
	}
	_, data, err := s.At(idx)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(data)
	return err
}

// doPick moves an entry to the top of the stack. If pickArg is a numeric
// index (or "first"/"last"), the move is performed non-interactively. If
// pickArg is empty and we have a TTY on both stdin and stderr, we launch
// the arrow-key picker; otherwise we no-op with a stderr notice.
func doPick(paths platform.Paths, pickArg string) error {
	s, err := stack.New(paths.StackDir())
	if err != nil {
		return err
	}
	n, err := s.Len()
	if err != nil {
		return err
	}
	if n == 0 {
		fmt.Fprintln(os.Stderr, "stack is empty")
		return nil
	}

	var idx int
	if pickArg != "" {
		idx, err = resolveIndex(pickArg, n)
		if err != nil {
			return err
		}
	} else {
		if !isTTY(os.Stdin) || !isTTY(os.Stderr) {
			fmt.Fprintln(os.Stderr, "yoink-n-yeet: --pick without an index requires a TTY; pass --pick N (or first/last)")
			return nil
		}
		ents, err := s.List()
		if err != nil {
			return err
		}
		items := make([]pick.Item, len(ents))
		for i, e := range ents {
			raw, _ := os.ReadFile(e.Path)
			items[i] = pick.Item{Label: fmt.Sprintf("[%d] %s  %s  %s",
				i,
				e.Created.Local().Format("15:04:05"),
				humanSize(e.Size),
				redact.Preview(string(raw), 60))}
		}
		picked, err := pick.Run(items, "Pick an entry to move to the top  (↑↓/jk move, ⏎ select, q cancel):")
		if err != nil {
			if errors.Is(err, pick.ErrCancelled) {
				return nil
			}
			return err
		}
		idx = picked
	}

	if err := s.MoveToTop(idx); err != nil {
		return err
	}
	// Mirror the new top to the OS clipboard so Cmd-V matches the stack.
	if _, top, err := s.Peek(); err == nil {
		if cb, err := clipboard.Detect(); err == nil {
			_ = cb.Copy(top)
		}
	}
	return nil
}

// resolveIndex parses a human-friendly index token into a 0-based index
// against a stack of length n. "" and "first" resolve to 0; "last" resolves
// to n-1; otherwise the token is parsed as a decimal integer.
func resolveIndex(tok string, n int) (int, error) {
	switch strings.ToLower(tok) {
	case "", "first", "top":
		return 0, nil
	case "last", "bottom":
		return n - 1, nil
	}
	idx, err := strconv.Atoi(tok)
	if err != nil {
		return -1, fmt.Errorf("invalid index %q (want an integer, \"first\", or \"last\")", tok)
	}
	if idx < 0 || idx >= n {
		return -1, fmt.Errorf("index %d out of range (valid: 0..%d)", idx, n-1)
	}
	return idx, nil
}

func doDrain(paths platform.Paths, o *opts) error {
	s, err := stack.New(paths.StackDir())
	if err != nil {
		return err
	}
	var (
		d   time.Duration
		all bool
	)
	switch {
	case o.drainDays > 0:
		d = time.Duration(o.drainDays) * 24 * time.Hour
	case o.drainHrs > 0:
		d = time.Duration(o.drainHrs) * time.Hour
	default:
		all = true
	}
	n, err := s.Len()
	if err != nil {
		return err
	}
	if n == 0 {
		fmt.Fprintln(os.Stderr, "stack is empty")
		return nil
	}
	prompt := fmt.Sprintf("drain all %d entries?", n)
	if d > 0 {
		prompt = fmt.Sprintf("drain entries older than %s?", d)
	}
	ok, err := promptYN(os.Stderr, prompt, false)
	if err != nil || !ok {
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "cancelled.")
		return nil
	}
	removed, err := s.Drain(d, all, true)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "removed %d entries\n", removed)
	// If we cleared everything, also clear the OS clipboard.
	if leftover, _ := s.Len(); leftover == 0 {
		if cb, err := clipboard.Detect(); err == nil {
			_ = cb.Copy(nil)
		}
	}
	return nil
}

func doStats(paths platform.Paths, asJSON bool) error {
	s, err := stats.Summarize(paths.StatsFile())
	if err != nil {
		return err
	}
	if asJSON {
		return json.NewEncoder(os.Stdout).Encode(s)
	}
	stats.WriteHuman(os.Stdout, s)
	return nil
}

func doAutoUpdate(paths platform.Paths, cfg *config.Config, mode string) error {
	switch mode {
	case "on":
		cfg.AutoUpdate = true
	case "off":
		cfg.AutoUpdate = false
	case "status":
		fmt.Printf("auto-update: %s\n", boolOnOff(cfg.AutoUpdate))
		return nil
	default:
		return fmt.Errorf("--auto-update: want on|off|status, got %q", mode)
	}
	if err := config.Save(paths.ConfigFile(), cfg); err != nil {
		return err
	}
	fmt.Printf("auto-update: %s\n", boolOnOff(cfg.AutoUpdate))
	return nil
}

// doReport creates a GitHub issue on the upstream repo. The happy path
// uses the `gh` CLI (fastest, no browser). If gh is missing, unauthenticated,
// or fails for any other reason, we fall back to opening the GitHub web
// issue-creation form in the user's default browser with the title and body
// prefilled via query parameters.
//
// Missing title or body is filled in via interactive prompts on a TTY, and
// treated as an error otherwise so scripts get a clear failure instead of
// a blocking read.
func doReport(title, body string) error {
	const upstream = "CoreyRDean/yoink-n-yeet"

	var err error
	if title == "" {
		title, err = promptLine(os.Stderr, "Issue title: ")
		if err != nil {
			return err
		}
	}
	if title == "" {
		return errors.New("--report: title is required")
	}
	if body == "" {
		body, err = promptLine(os.Stderr, "Issue body (single line, blank to skip): ")
		if err != nil {
			return err
		}
	}

	// Prefer gh CLI when available and authenticated.
	if _, lookErr := exec.LookPath("gh"); lookErr == nil {
		auth := exec.Command("gh", "auth", "status")
		auth.Stdout = io.Discard
		auth.Stderr = io.Discard
		if authErr := auth.Run(); authErr == nil {
			args := []string{"issue", "create", "--repo", upstream, "--title", title}
			if body != "" {
				args = append(args, "--body", body)
			} else {
				args = append(args, "--body", "")
			}
			cmd := exec.Command("gh", args...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if runErr := cmd.Run(); runErr == nil {
				return nil
			}
			// gh ran but errored (network, perms, etc.) — fall through to browser.
			fmt.Fprintln(os.Stderr, "gh issue create failed; falling back to browser")
		}
	}

	// Fallback: open the browser-based issue form with the fields prefilled.
	v := url.Values{}
	v.Set("title", title)
	if body != "" {
		v.Set("body", body)
	}
	link := "https://github.com/" + upstream + "/issues/new?" + v.Encode()
	fmt.Fprintf(os.Stderr, "opening %s\n", link)
	return openBrowser(link)
}

// openBrowser launches the platform's default URL handler. Best-effort: if
// the platform is exotic or no handler is available, the URL is printed so
// the user can copy it.
func openBrowser(link string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", link)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", link)
	default:
		if _, err := exec.LookPath("xdg-open"); err == nil {
			cmd = exec.Command("xdg-open", link)
		} else {
			fmt.Fprintln(os.Stderr, "no browser launcher found; open this URL manually:")
			fmt.Fprintln(os.Stderr, link)
			return nil
		}
	}
	// Detach so the CLI can exit immediately; stdout/stderr suppressed so
	// the browser process doesn't leak noise onto the terminal.
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Start()
}

// promptLine reads a single line from stdin, emitting msg as the prompt.
// Returns an error with a helpful message when stdin is not a TTY, so scripts
// that forgot to pass a title or body fail fast rather than blocking on a
// closed pipe.
func promptLine(w io.Writer, msg string) (string, error) {
	if !isTTY(os.Stdin) {
		return "", errors.New("stdin is not a TTY; supply title and body as --report args")
	}
	fmt.Fprint(w, msg)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func doUninstall(cfg *config.Config) error {
	// Prefer the bundled uninstall.sh from a local install; fall back to
	// fetching it from the repo.
	var script string
	if cfg.Channel == "local" && cfg.LocalRepoPath != "" {
		candidate := filepath.Join(cfg.LocalRepoPath, "uninstall.sh")
		if _, err := os.Stat(candidate); err == nil {
			script = candidate
		}
	}
	if script == "" {
		url := fmt.Sprintf("https://raw.githubusercontent.com/%s/main/uninstall.sh", update.Repo)
		path, err := update.FetchInstaller(url)
		if err != nil {
			return fmt.Errorf("fetch uninstaller: %w", err)
		}
		defer os.Remove(path)
		script = path
	}
	cmd := exec.Command("/bin/sh", script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// ---------- helpers ----------

func isTTY(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func promptYN(w io.Writer, msg string, defaultYes bool) (bool, error) {
	suffix := " [y/N] "
	if defaultYes {
		suffix = " [Y/n] "
	}
	fmt.Fprint(w, msg+suffix)
	if !isTTY(os.Stdin) {
		// Non-interactive: choose the safe default, which is "no".
		fmt.Fprintln(w, "(non-interactive; defaulting to no)")
		return false, nil
	}
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	ans := strings.ToLower(strings.TrimSpace(line))
	if ans == "" {
		return defaultYes, nil
	}
	return ans == "y" || ans == "yes", nil
}

func confirmDryPush(data []byte, source string) (bool, error) {
	preview := redact.Preview(string(data), 80)
	fmt.Fprintf(os.Stderr, "would push %d byte(s) from %q:\n  %s\n", len(data), source, preview)
	return promptYN(os.Stderr, "push this entry?", true)
}

func formatDryPopPreview(e *stack.Entry, data []byte, width int) string {
	pv := redact.Preview(string(data), width)
	return fmt.Sprintf("top-of-stack: %s  %s\n  %s",
		e.Created.Local().Format("2006-01-02 15:04:05"), humanSize(e.Size), pv)
}

func humanSize(n int64) string {
	const k = 1024
	switch {
	case n < k:
		return fmt.Sprintf("%dB", n)
	case n < k*k:
		return fmt.Sprintf("%.1fKiB", float64(n)/k)
	case n < k*k*k:
		return fmt.Sprintf("%.1fMiB", float64(n)/(k*k))
	default:
		return fmt.Sprintf("%.1fGiB", float64(n)/(k*k*k))
	}
}

func boolOnOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

func printHelp(w io.Writer) {
	fmt.Fprintln(w, `yoink-n-yeet — a clipboard stack

Usage:
  yoink / yk  <cmd> [args]    run <cmd>, push stdout
  ... | yoink / yk            push stdin
  yeet / yt                   pop top of stack to stdout

Flags (valid on either name):
  --list [--json]             list stack (0 = top)
  --show [N|first|last]       print entry N (default: first/top) to stdout
                              (no consumption)
  --peek                      alias for --show first
  --pick [N|first|last]       move entry N to the top of the stack; with no
                              arg launches an arrow-key picker on a TTY and
                              no-ops otherwise
  --dry                       preview + interactive prompt
  --drain, --clear            wipe the stack (confirms)
  --drain --days N            drop entries older than N days
  --drain --hours N           drop entries older than N hours
  --stats [--json]            usage summary
  --doctor                    platform + clipboard diagnostics
  --version                   show version + channel
  --update [stable|nightly]   self-update (stable is default)
  --stable                    shortcut for --update stable
  --auto-update on|off|status toggle background auto-update (default off)
  --no-update-check           skip the async update check this run
  --uninstall                 remove the binary and symlinks
  -c, --cb                    on yoink/yk: import the current OS clipboard
                              onto the stack (source="clipboard");
                              on yeet/yt: emit the OS clipboard to stdout
                              without touching the stack
  --report [title] [body]     file a bug/feature issue on the upstream repo
                              (uses gh CLI if available and authed;
                              otherwise opens the GitHub issue form in
                              your browser with fields prefilled)

See https://github.com/CoreyRDean/yoink-n-yeet`)
}

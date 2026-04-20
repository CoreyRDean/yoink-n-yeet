// Package redact replaces common secret-shaped substrings with a placeholder
// so that --list previews are safer to display in a terminal that might be
// screen-shared or logged.
//
// Redaction is best-effort. Users who want to see the actual bytes should
// use --show N or --peek, which never redact.
package redact

import (
	"regexp"
	"strings"
)

type rule struct {
	name string
	re   *regexp.Regexp
}

// Patterns are deliberately conservative; false positives are fine,
// false negatives in --list previews leak secrets.
var patterns = []rule{
	{"aws-key-id", regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
	{"gh-token", regexp.MustCompile(`(?i)\b(?:gh[pousr]_[A-Za-z0-9]{36,255})\b`)},
	{"slack-token", regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]{10,72}`)},
	{"jwt", regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`)},
	{"pem-private", regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z ]*PRIVATE KEY-----`)},
	{"openai", regexp.MustCompile(`sk-[A-Za-z0-9_-]{20,}`)},
	{"hex-64", regexp.MustCompile(`\b[a-f0-9]{64,}\b`)}, // sha256 / long hex secrets
	{"kv-secret", regexp.MustCompile(`(?i)\b(?:password|passwd|secret|api[_-]?key|token|authorization)\s*[:=]\s*\S+`)},
}

// Apply returns s with matched secrets replaced by a ⟨redacted:name⟩ marker.
func Apply(s string) string {
	for _, p := range patterns {
		s = p.re.ReplaceAllString(s, "⟨redacted:"+p.name+"⟩")
	}
	return s
}

// Preview trims s to a single-line snippet of at most width runes, escaping
// control characters and applying redaction. It is intended for --list.
func Preview(s string, width int) string {
	if width <= 0 {
		width = 80
	}
	// Collapse to a single line and strip control chars before measuring.
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\n' || r == '\r':
			b.WriteRune(' ')
		case r == '\t':
			b.WriteRune(' ')
		case r < 0x20:
			b.WriteRune('·')
		default:
			b.WriteRune(r)
		}
	}
	line := Apply(b.String())
	line = strings.TrimSpace(line)
	if runesLen(line) > width {
		return truncateRunes(line, width-1) + "…"
	}
	return line
}

func runesLen(s string) int { return len([]rune(s)) }

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

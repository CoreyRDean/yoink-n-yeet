package redact

import (
	"strings"
	"testing"
)

func TestApplyRedactsCommonSecrets(t *testing.T) {
	cases := []struct {
		name, in string
	}{
		{"aws", "AKIAIOSFODNN7EXAMPLE is an access key"},
		{"gh-token", "token=ghp_abcdefghijklmnopqrstuvwxyz01234567890AB"},
		{"slack", "xoxb-1234567890-abcdefghij"},
		{"openai", "OPENAI_API_KEY=sk-abcdefghijklmnopqrstuvwxyz123456"},
		{"kv-secret", "password: hunter2"},
		{"jwt", "Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.eyJ1c2VyIjoiYWxpY2UifQ.signature12345"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := Apply(c.in)
			if !strings.Contains(out, "⟨redacted:") {
				t.Fatalf("expected redaction marker in %q, got %q", c.in, out)
			}
		})
	}
}

func TestApplyLeavesBenignTextAlone(t *testing.T) {
	in := "the quick brown fox jumps over 12345 lazy dogs"
	if got := Apply(in); got != in {
		t.Fatalf("unexpected mutation: %q -> %q", in, got)
	}
}

func TestPreviewCollapsesControlChars(t *testing.T) {
	in := "line1\nline2\ttabbed"
	got := Preview(in, 40)
	if strings.ContainsAny(got, "\n\t") {
		t.Fatalf("preview still contains control chars: %q", got)
	}
}

func TestPreviewTruncates(t *testing.T) {
	// Use a non-hex-safe char so the string doesn't trip the hex-64 rule.
	in := strings.Repeat("Z", 200)
	got := Preview(in, 20)
	if runesLen(got) > 20 {
		t.Fatalf("preview exceeded width: %d runes in %q", runesLen(got), got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("expected ellipsis suffix, got %q", got)
	}
}

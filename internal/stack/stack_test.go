package stack

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestStack(t *testing.T) *Stack {
	t.Helper()
	dir := t.TempDir()
	s, err := New(filepath.Join(dir, "stack"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

func TestPushPeekPopOrder(t *testing.T) {
	s := newTestStack(t)
	for _, w := range []string{"first", "second", "third"} {
		if _, err := s.Push([]byte(w), "test"); err != nil {
			t.Fatalf("push %q: %v", w, err)
		}
	}
	if n, _ := s.Len(); n != 3 {
		t.Fatalf("Len = %d, want 3", n)
	}

	// Peek returns top (most recent) without popping.
	_, top, err := s.Peek()
	if err != nil {
		t.Fatalf("peek: %v", err)
	}
	if string(top) != "third" {
		t.Fatalf("peek = %q, want %q", top, "third")
	}
	if n, _ := s.Len(); n != 3 {
		t.Fatalf("Len after peek = %d, want 3", n)
	}

	// Pop returns top-to-bottom: third, second, first.
	wantOrder := []string{"third", "second", "first"}
	for _, want := range wantOrder {
		_, got, err := s.Pop()
		if err != nil {
			t.Fatalf("pop: %v", err)
		}
		if string(got) != want {
			t.Fatalf("pop = %q, want %q", got, want)
		}
	}

	// Stack is now empty.
	if _, _, err := s.Pop(); !errors.Is(err, ErrEmpty) {
		t.Fatalf("pop empty: err = %v, want %v", err, ErrEmpty)
	}
}

func TestBinarySafety(t *testing.T) {
	s := newTestStack(t)
	payload := []byte{0x00, 0x01, 0xff, 0xfe, '\n', 'a', 0xc3, 0xa9} // NUL, CRLF-ish, UTF-8 é
	if _, err := s.Push(payload, "bytes"); err != nil {
		t.Fatalf("push: %v", err)
	}
	_, got, err := s.Pop()
	if err != nil {
		t.Fatalf("pop: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("payload mismatch\n want: %v\n  got: %v", payload, got)
	}
}

func TestAtIndex(t *testing.T) {
	s := newTestStack(t)
	for _, w := range []string{"bot", "mid", "top"} {
		if _, err := s.Push([]byte(w), "t"); err != nil {
			t.Fatalf("push: %v", err)
		}
	}
	cases := []struct {
		idx  int
		want string
	}{{0, "top"}, {1, "mid"}, {2, "bot"}}
	for _, c := range cases {
		_, got, err := s.At(c.idx)
		if err != nil {
			t.Fatalf("at %d: %v", c.idx, err)
		}
		if string(got) != c.want {
			t.Fatalf("at %d = %q, want %q", c.idx, got, c.want)
		}
	}
	if _, _, err := s.At(99); !errors.Is(err, ErrOutOfRange) {
		t.Fatalf("at out-of-range: err = %v, want %v", err, ErrOutOfRange)
	}
	if _, _, err := s.At(-1); !errors.Is(err, ErrOutOfRange) {
		t.Fatalf("at -1: err = %v, want %v", err, ErrOutOfRange)
	}
}

func TestDrainAll(t *testing.T) {
	s := newTestStack(t)
	for i := 0; i < 5; i++ {
		_, _ = s.Push([]byte("x"), "t")
	}
	removed, err := s.Drain(0, true, false)
	if err != nil {
		t.Fatalf("drain: %v", err)
	}
	if removed != 5 {
		t.Fatalf("drain removed = %d, want 5", removed)
	}
	if n, _ := s.Len(); n != 0 {
		t.Fatalf("len after drain = %d, want 0", n)
	}
}

func TestDrainOlderThan(t *testing.T) {
	s := newTestStack(t)
	// Push a fresh entry.
	_, err := s.Push([]byte("keep"), "t")
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	// Push a second entry and then backdate its mtime + metadata to simulate age.
	old, err := s.Push([]byte("drop"), "t")
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	older := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(old.Path, older, older); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	// Rewrite the metadata file with a created timestamp 48h ago.
	oldJSON := `{"id":"` + old.ID + `","created":"` + older.UTC().Format(time.RFC3339Nano) + `","size":4,"source":"t"}`
	if err := os.WriteFile(old.MetaPath, []byte(oldJSON), 0o600); err != nil {
		t.Fatalf("rewrite meta: %v", err)
	}

	removed, err := s.Drain(24*time.Hour, false, false)
	if err != nil {
		t.Fatalf("drain: %v", err)
	}
	if removed != 1 {
		t.Fatalf("drain removed = %d, want 1", removed)
	}
	// Only the fresh one should remain.
	_, got, err := s.Peek()
	if err != nil {
		t.Fatalf("peek after drain: %v", err)
	}
	if string(got) != "keep" {
		t.Fatalf("peek = %q, want keep", got)
	}
}

func TestPushWithMissingMetaSynthesizes(t *testing.T) {
	s := newTestStack(t)
	e, err := s.Push([]byte("payload"), "t")
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	// Nuke the metadata; list() should still return the entry.
	if err := os.Remove(e.MetaPath); err != nil {
		t.Fatalf("rm meta: %v", err)
	}
	ents, err := s.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(ents) != 1 {
		t.Fatalf("list len = %d, want 1", len(ents))
	}
	if ents[0].Size != 7 {
		t.Fatalf("synthesized size = %d, want 7", ents[0].Size)
	}
}

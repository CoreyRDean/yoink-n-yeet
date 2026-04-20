// Package stack implements the persistent clipboard stack.
//
// Entries are stored as two files per push:
//
//	<unix_nanos>.bin   raw bytes
//	<unix_nanos>.json  metadata (created, size, sha256, source, tags)
//
// The timestamp-encoded filename lets us list, age-filter, and order
// entries without ever opening a payload. Entries are sorted by filename
// (lexicographic == chronological because nanoseconds are zero-padded).
//
// Index 0 is the top of the stack (the most recent push), matching the
// user's mental model of "what did I last put here, and what's under it?"
package stack

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Entry is the public handle to a stack entry. Callers get Path to read the
// raw bytes and MetaPath for the JSON metadata sidecar.
type Entry struct {
	ID       string    `json:"id"`       // filename stem (zero-padded unix nanos)
	Created  time.Time `json:"created"`  // wall-clock creation time
	Size     int64     `json:"size"`     // payload byte count
	SHA256   string    `json:"sha256"`   // hex-encoded sha256 of payload
	Source   string    `json:"source"`   // command that produced it, or "stdin"
	Tags     []string  `json:"tags,omitempty"`
	Path     string    `json:"-"` // absolute path to .bin
	MetaPath string    `json:"-"` // absolute path to .json
}

// Stack is rooted at a directory that holds the .bin/.json pairs.
type Stack struct {
	dir string
}

// New returns a Stack backed by dir. The directory is created if missing.
func New(dir string) (*Stack, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Stack{dir: dir}, nil
}

// ErrEmpty is returned by Pop/Peek when the stack has no entries.
var ErrEmpty = errors.New("stack is empty")

// ErrOutOfRange is returned by At when idx is beyond the stack.
var ErrOutOfRange = errors.New("index out of range")

// Len returns the number of entries currently in the stack.
func (s *Stack) Len() (int, error) {
	ents, err := s.list()
	if err != nil {
		return 0, err
	}
	return len(ents), nil
}

// Push writes data as a new top-of-stack entry and returns the created
// Entry handle. source describes what produced the data (e.g. "cat file"
// or "stdin") and is stored in the metadata sidecar.
func (s *Stack) Push(data []byte, source string) (*Entry, error) {
	// Use unix nanos + 4 hex bytes of entropy to avoid collisions when two
	// pushes happen in the same nanosecond.
	var nonce [4]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	// Pad to 19 digits so lexicographic sort == chronological sort forever.
	id := fmt.Sprintf("%019d-%s", now.UnixNano(), hex.EncodeToString(nonce[:]))
	binPath := filepath.Join(s.dir, id+".bin")
	metaPath := filepath.Join(s.dir, id+".json")

	if err := writeFileAtomic(binPath, data, 0o600); err != nil {
		return nil, fmt.Errorf("write bin: %w", err)
	}
	sum := sha256.Sum256(data)
	entry := &Entry{
		ID:       id,
		Created:  now,
		Size:     int64(len(data)),
		SHA256:   hex.EncodeToString(sum[:]),
		Source:   source,
		Path:     binPath,
		MetaPath: metaPath,
	}
	meta, _ := json.MarshalIndent(entry, "", "  ")
	if err := writeFileAtomic(metaPath, append(meta, '\n'), 0o600); err != nil {
		_ = os.Remove(binPath)
		return nil, fmt.Errorf("write meta: %w", err)
	}
	return entry, nil
}

// Peek returns the top entry and its bytes without removing it.
func (s *Stack) Peek() (*Entry, []byte, error) {
	ents, err := s.list()
	if err != nil {
		return nil, nil, err
	}
	if len(ents) == 0 {
		return nil, nil, ErrEmpty
	}
	return s.read(ents[0])
}

// At returns the entry at index idx (0 = top) and its bytes. It does not
// remove the entry.
func (s *Stack) At(idx int) (*Entry, []byte, error) {
	if idx < 0 {
		return nil, nil, ErrOutOfRange
	}
	ents, err := s.list()
	if err != nil {
		return nil, nil, err
	}
	if idx >= len(ents) {
		return nil, nil, ErrOutOfRange
	}
	return s.read(ents[idx])
}

// Pop removes and returns the top entry and its bytes.
func (s *Stack) Pop() (*Entry, []byte, error) {
	e, data, err := s.Peek()
	if err != nil {
		return nil, nil, err
	}
	if err := os.Remove(e.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, nil, err
	}
	if err := os.Remove(e.MetaPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, nil, err
	}
	return e, data, nil
}

// List returns every entry, top first. Bytes are not loaded.
func (s *Stack) List() ([]*Entry, error) { return s.list() }

// Drain removes entries. If all is true, every entry is removed. If
// olderThan > 0, only entries created more than olderThan ago are removed.
// When both are supplied, olderThan takes effect and all is ignored.
// If secure is true, payload bytes are overwritten with zeros before
// unlink. Returns the number of entries removed.
func (s *Stack) Drain(olderThan time.Duration, all, secure bool) (int, error) {
	ents, err := s.list()
	if err != nil {
		return 0, err
	}
	cutoff := time.Now().UTC().Add(-olderThan)
	removed := 0
	for _, e := range ents {
		switch {
		case olderThan > 0:
			if !e.Created.Before(cutoff) {
				continue
			}
		case !all:
			continue
		}
		if secure {
			// The metadata sidecar stores the Source command, which often
			// leaks as much as the payload (e.g. an AWS CLI invocation that
			// names a secret). Zero both files, not just the payload.
			_ = overwrite(e.Path, e.Size)
			_ = overwrite(e.MetaPath, 0)
		}
		if err := os.Remove(e.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return removed, err
		}
		if err := os.Remove(e.MetaPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

// Dir returns the directory the stack is rooted at (useful for tests and
// debug output).
func (s *Stack) Dir() string { return s.dir }

// MoveToTop moves the entry at idx to the top of the stack (index 0).
// It renames both files to a fresh top-sort-order ID while preserving the
// original Created timestamp in the metadata. This is what a user expects
// from "move to top": the entry sorts first now, but age-based filters
// like --drain --days N still see it as old.
func (s *Stack) MoveToTop(idx int) error {
	if idx == 0 {
		return nil
	}
	ents, err := s.list()
	if err != nil {
		return err
	}
	if idx >= len(ents) {
		return ErrOutOfRange
	}
	e := ents[idx]

	// Generate a fresh top-sort ID the same way Push does.
	var nonce [4]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	newID := fmt.Sprintf("%019d-%s", time.Now().UTC().UnixNano(), hex.EncodeToString(nonce[:]))
	newBin := filepath.Join(s.dir, newID+".bin")
	newMeta := filepath.Join(s.dir, newID+".json")

	// Rename bin first; if that fails we've changed nothing.
	if err := os.Rename(e.Path, newBin); err != nil {
		return err
	}
	// Meta rename second; roll bin back on failure so we don't orphan files.
	if err := os.Rename(e.MetaPath, newMeta); err != nil {
		_ = os.Rename(newBin, e.Path)
		return err
	}

	// Patch the metadata so its ID field matches the new filename. Created,
	// Source, SHA256, Size, Tags are all preserved.
	raw, readErr := os.ReadFile(newMeta)
	if readErr != nil {
		return nil // metadata is desynced but the entry is usable; log nothing
	}
	var meta Entry
	if err := json.Unmarshal(raw, &meta); err != nil {
		return nil
	}
	meta.ID = newID
	newRaw, err := json.MarshalIndent(&meta, "", "  ")
	if err != nil {
		return nil
	}
	_ = writeFileAtomic(newMeta, append(newRaw, '\n'), 0o600)
	return nil
}

// ----- internals -----

// list enumerates entries, top first.
func (s *Stack) list() ([]*Entry, error) {
	dirents, err := os.ReadDir(s.dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var stems []string
	for _, de := range dirents {
		if de.IsDir() {
			continue
		}
		n := de.Name()
		if !strings.HasSuffix(n, ".bin") {
			continue
		}
		stems = append(stems, strings.TrimSuffix(n, ".bin"))
	}
	// Descending sort = top (newest) first.
	sort.Sort(sort.Reverse(sort.StringSlice(stems)))

	ents := make([]*Entry, 0, len(stems))
	for _, stem := range stems {
		e, err := s.loadMeta(stem)
		if err != nil {
			// Metadata missing or corrupt: synthesize from filename so the
			// entry is still usable.
			e = synthesizeEntry(s.dir, stem)
		}
		ents = append(ents, e)
	}
	return ents, nil
}

func (s *Stack) loadMeta(stem string) (*Entry, error) {
	metaPath := filepath.Join(s.dir, stem+".json")
	raw, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, err
	}
	var e Entry
	if err := json.Unmarshal(raw, &e); err != nil {
		return nil, err
	}
	e.Path = filepath.Join(s.dir, stem+".bin")
	e.MetaPath = metaPath
	return &e, nil
}

func synthesizeEntry(dir, stem string) *Entry {
	binPath := filepath.Join(dir, stem+".bin")
	e := &Entry{
		ID:       stem,
		Path:     binPath,
		MetaPath: filepath.Join(dir, stem+".json"),
	}
	// Parse the unix-nanos prefix of the stem for a best-effort Created time.
	if dash := strings.IndexByte(stem, '-'); dash > 0 {
		if n, err := strconv.ParseInt(stem[:dash], 10, 64); err == nil {
			e.Created = time.Unix(0, n).UTC()
		}
	}
	if info, err := os.Stat(binPath); err == nil {
		e.Size = info.Size()
	}
	return e
}

func (s *Stack) read(e *Entry) (*Entry, []byte, error) {
	data, err := os.ReadFile(e.Path)
	if err != nil {
		return nil, nil, err
	}
	return e, data, nil
}

// writeFileAtomic writes data to path via a temp file + rename. This avoids
// partial files if the process is interrupted mid-write, which matters for
// a stack that you'll rely on tomorrow.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".yoink-tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	return os.Rename(tmpName, path)
}

// overwrite best-effort zeros the first size bytes of path. Cross-platform
// secure deletion is a hard problem; this is intentionally a best effort
// aimed at defeating casual recovery.
func overwrite(path string, size int64) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if size <= 0 {
		info, err := f.Stat()
		if err != nil {
			return err
		}
		size = info.Size()
	}
	zero := make([]byte, 4096)
	for written := int64(0); written < size; {
		n := int64(len(zero))
		if rem := size - written; rem < n {
			n = rem
		}
		if _, err := f.Write(zero[:n]); err != nil {
			return err
		}
		written += n
	}
	return f.Sync()
}

// CopyTo writes the entry's bytes to w. Convenience for callers that want
// to stream straight to stdout without buffering through a slice.
func (s *Stack) CopyTo(e *Entry, w io.Writer) error {
	f, err := os.Open(e.Path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(w, f)
	return err
}

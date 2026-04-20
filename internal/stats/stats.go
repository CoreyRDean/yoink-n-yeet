// Package stats records push/pop events to an append-only JSONL file and
// produces summaries on demand. Events are append-only to keep writes
// O(1) and crash-safe; the summary is computed by a single pass at --stats
// time, which is plenty fast for any reasonable corpus.
package stats

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"time"
)

// Op is the type of event being recorded.
type Op string

const (
	OpPush Op = "push"
	OpPop  Op = "pop"
)

// Event is a single recorded stats row.
type Event struct {
	Time   time.Time `json:"t"`           // event wall clock
	Op     Op        `json:"op"`          // push or pop
	Size   int64     `json:"size"`        // bytes of payload
	Source string    `json:"src,omitempty"` // command source (push only)
	AgeMs  int64     `json:"age_ms,omitempty"` // how long the popped entry lived (pop only)
}

// Append writes e as a single JSON line to path. If path does not exist it
// is created. Errors here are best-effort; stats must never block a user.
func Append(path string, e Event) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	raw, err := json.Marshal(e)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	_, err = f.Write(raw)
	return err
}

// Summary is the rolled-up view of the stats log at a point in time.
type Summary struct {
	Pushes         int64            `json:"pushes"`
	Pops           int64            `json:"pops"`
	BytesPushed    int64            `json:"bytes_pushed"`
	BytesPopped    int64            `json:"bytes_popped"`
	FirstEvent     *time.Time       `json:"first_event,omitempty"`
	LastEvent      *time.Time       `json:"last_event,omitempty"`
	AvgAgeAtPopMs  float64          `json:"avg_age_at_pop_ms,omitempty"`
	TopSources     []SourceCount    `json:"top_sources,omitempty"`
	PerDay         map[string]int64 `json:"per_day,omitempty"`
}

// SourceCount pairs a command source with its push count.
type SourceCount struct {
	Source string `json:"source"`
	Count  int64  `json:"count"`
}

// Summarize scans path and rolls the events into a Summary. Missing file
// yields a zero-value Summary and nil error.
func Summarize(path string) (*Summary, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Summary{PerDay: map[string]int64{}}, nil
		}
		return nil, err
	}
	defer f.Close()

	s := &Summary{PerDay: map[string]int64{}}
	srcCounts := map[string]int64{}
	var ageSum, ageN int64

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 2*1024*1024)
	for sc.Scan() {
		var e Event
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			continue // tolerate the occasional garbled line
		}
		if s.FirstEvent == nil || e.Time.Before(*s.FirstEvent) {
			t := e.Time
			s.FirstEvent = &t
		}
		if s.LastEvent == nil || e.Time.After(*s.LastEvent) {
			t := e.Time
			s.LastEvent = &t
		}
		day := e.Time.Format("2006-01-02")
		s.PerDay[day]++
		switch e.Op {
		case OpPush:
			s.Pushes++
			s.BytesPushed += e.Size
			if e.Source != "" {
				srcCounts[e.Source]++
			}
		case OpPop:
			s.Pops++
			s.BytesPopped += e.Size
			if e.AgeMs > 0 {
				ageSum += e.AgeMs
				ageN++
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if ageN > 0 {
		s.AvgAgeAtPopMs = float64(ageSum) / float64(ageN)
	}
	// Top 10 sources.
	for src, n := range srcCounts {
		s.TopSources = append(s.TopSources, SourceCount{Source: src, Count: n})
	}
	sort.Slice(s.TopSources, func(i, j int) bool {
		if s.TopSources[i].Count != s.TopSources[j].Count {
			return s.TopSources[i].Count > s.TopSources[j].Count
		}
		return s.TopSources[i].Source < s.TopSources[j].Source
	})
	if len(s.TopSources) > 10 {
		s.TopSources = s.TopSources[:10]
	}
	return s, nil
}

// WriteHuman renders s as a readable summary to w.
func WriteHuman(w io.Writer, s *Summary) {
	fmt.Fprintf(w, "Pushes: %d  (%s)\n", s.Pushes, humanBytes(s.BytesPushed))
	fmt.Fprintf(w, "Pops:   %d  (%s)\n", s.Pops, humanBytes(s.BytesPopped))
	if s.AvgAgeAtPopMs > 0 {
		fmt.Fprintf(w, "Avg age at pop: %s\n", time.Duration(s.AvgAgeAtPopMs*float64(time.Millisecond)).Round(time.Second))
	}
	if s.FirstEvent != nil && s.LastEvent != nil {
		fmt.Fprintf(w, "Window: %s → %s\n",
			s.FirstEvent.Local().Format("2006-01-02 15:04:05"),
			s.LastEvent.Local().Format("2006-01-02 15:04:05"))
	}
	if len(s.TopSources) > 0 {
		fmt.Fprintln(w, "Top sources:")
		for _, sc := range s.TopSources {
			fmt.Fprintf(w, "  %6d  %s\n", sc.Count, sc.Source)
		}
	}
	if len(s.PerDay) > 0 {
		// Last 7 days, chronological.
		var days []string
		for d := range s.PerDay {
			days = append(days, d)
		}
		sort.Strings(days)
		if len(days) > 7 {
			days = days[len(days)-7:]
		}
		fmt.Fprintln(w, "Recent activity:")
		for _, d := range days {
			fmt.Fprintf(w, "  %s  %d\n", d, s.PerDay[d])
		}
	}
}

func humanBytes(n int64) string {
	const kb = 1024
	switch {
	case n < kb:
		return fmt.Sprintf("%d B", n)
	case n < kb*kb:
		return fmt.Sprintf("%.1f KiB", float64(n)/kb)
	case n < kb*kb*kb:
		return fmt.Sprintf("%.1f MiB", float64(n)/(kb*kb))
	default:
		return fmt.Sprintf("%.1f GiB", float64(n)/(kb*kb*kb))
	}
}


package node

import (
	"testing"
	"time"
)

// StatsWindow must count only the samples inside the requested window, while
// Stats keeps the fixed 24h bucket semantics.
func TestSlidingWindow_StatsWindow(t *testing.T) {
	w := NewSlidingWindow(DefaultProbeWindowSize)
	now := time.Now()

	w.Record(true, now.Add(-40*time.Minute))
	w.Record(false, now.Add(-20*time.Minute))
	w.Record(true, now.Add(-5*time.Minute))

	// A 30m window sees the last two samples: 1 pass of 2.
	succ, total := w.StatsWindow(now, 30*time.Minute)
	if succ != 1 || total != 2 {
		t.Fatalf("30m window = %d/%d, want 1/2", succ, total)
	}

	// A 1h window sees all three.
	succ, total = w.StatsWindow(now, time.Hour)
	if succ != 2 || total != 3 {
		t.Fatalf("1h window = %d/%d, want 2/3", succ, total)
	}

	// An empty window reports 0 samples so callers can treat it as "no data".
	if _, total := w.StatsWindow(now, time.Minute); total != 0 {
		t.Fatalf("1m window total = %d, want 0", total)
	}

	// A non-positive window falls back to the 24h view.
	succ, total = w.StatsWindow(now, 0)
	if succ != 2 || total != 3 {
		t.Fatalf("fallback = %d/%d, want 2/3", succ, total)
	}
}

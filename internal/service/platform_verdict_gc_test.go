package service

import (
	"testing"
	"time"

	"github.com/Resinat/Resin/internal/platform"
)

// The ledger pruner relies on GC(cutoff, live): a row is dropped only when its IP
// has left the pool *and* its last sighting is older than the grace window.
func TestVerdictStore_GCPrunesAbsentIPsAfterGrace(t *testing.T) {
	now := time.Now()
	store := NewVerdictStore(nil)
	store.Record("p1", "203.0.113.9", "hash", true, 200, now, platform.VerdictConfig{})

	absent := func(string, string) bool { return false }

	// Still inside the 5-minute grace: kept.
	if n, err := store.GC(now.Add(-5*time.Minute), absent); err != nil || n != 0 {
		t.Fatalf("inside grace: removed=%d err=%v, want 0", n, err)
	}
	if got := len(store.Snapshot("p1")); got != 1 {
		t.Fatalf("inside grace: rows=%d, want 1", got)
	}

	// Past the grace window and absent from the pool: dropped.
	if n, err := store.GC(now.Add(5*time.Minute+time.Second), absent); err != nil || n != 1 {
		t.Fatalf("past grace: removed=%d err=%v, want 1", n, err)
	}
	if got := len(store.Snapshot("p1")); got != 0 {
		t.Fatalf("past grace: rows=%d, want 0", got)
	}

	// Old row but the IP is still present: never dropped.
	store.Record("p1", "203.0.113.10", "hash", true, 200, now, platform.VerdictConfig{})
	present := func(_, ip string) bool { return ip == "203.0.113.10" }
	if n, err := store.GC(now.Add(time.Hour), present); err != nil || n != 0 {
		t.Fatalf("present: removed=%d err=%v, want 0", n, err)
	}
	if got := len(store.Snapshot("p1")); got != 1 {
		t.Fatalf("present: rows=%d, want 1", got)
	}
}

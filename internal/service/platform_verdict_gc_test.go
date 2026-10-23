package service

import (
	"testing"
	"time"

	"github.com/Resinat/Resin/internal/platform"
)

func TestVerdictStore_GC_RespectsRetentionAndLiveIPs(t *testing.T) {
	store := NewVerdictStore(nil)
	now := time.Now()
	retention := 72 * time.Hour
	cutoff := now.Add(-retention)

	// Live IP seen from the start (stale by LastSeen, still on a node).
	store.Record("p1", "10.0.0.1", "h1", true, 200, now.Add(-100*time.Hour), platform.VerdictConfig{Enabled: true, Threshold: 3, RehabSuccessThreshold: 3})
	// Gone IP, older than retention.
	store.Record("p1", "10.0.0.2", "h2", true, 200, now.Add(-100*time.Hour), platform.VerdictConfig{Enabled: true, Threshold: 3, RehabSuccessThreshold: 3})
	// Gone IP, inside the retention window.
	store.Record("p1", "10.0.0.3", "h3", true, 200, now.Add(-10*time.Hour), platform.VerdictConfig{Enabled: true, Threshold: 3, RehabSuccessThreshold: 3})

	removed, err := store.GC(cutoff, func(_, ip string) bool { return ip == "10.0.0.1" })
	if err != nil {
		t.Fatalf("GC: %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}

	rows := store.Snapshot("p1")
	got := make(map[string]bool, len(rows))
	for _, r := range rows {
		got[r.IP] = true
	}
	if !got["10.0.0.1"] {
		t.Fatal("live IP must survive GC")
	}
	if got["10.0.0.2"] {
		t.Fatal("stale, non-live IP must be removed")
	}
	if !got["10.0.0.3"] {
		t.Fatal("IP inside retention window must survive")
	}
}

func TestVerdictStore_GC_ReappearanceStartsUnknown(t *testing.T) {
	store := NewVerdictStore(nil)
	now := time.Now()

	store.Record("p1", "10.0.0.9", "h1", false, 200, now.Add(-100*time.Hour), platform.VerdictConfig{Enabled: true, Threshold: 3, RehabSuccessThreshold: 3})
	if _, err := store.GC(now.Add(-72*time.Hour), func(_, _ string) bool { return false }); err != nil {
		t.Fatalf("GC: %v", err)
	}
	if state, _, fails := store.EntryState("p1", "10.0.0.9"); state != "" || fails != 0 {
		t.Fatalf("after GC state=%q fails=%d, want empty/0", state, fails)
	}

	// Coming back creates a fresh unknown row.
	store.Record("p1", "10.0.0.9", "h1", true, 200, now, platform.VerdictConfig{Enabled: true, Threshold: 3, RehabSuccessThreshold: 3})
	state, _, _ := store.EntryState("p1", "10.0.0.9")
	if state != "normal" {
		t.Fatalf("reappeared IP state = %q, want normal", state)
	}
}

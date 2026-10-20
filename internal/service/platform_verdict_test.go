package service

import (
	"net/netip"
	"testing"
	"time"

	"github.com/Resinat/Resin/internal/platform"
)

func TestVerdictStore_BlacklistAndRehab(t *testing.T) {
	store := NewVerdictStore(nil)
	cfg := platform.VerdictConfig{Enabled: true, Threshold: 3, RehabSuccessThreshold: 3}
	ip := "1.2.3.4"
	now := time.Now()

	for i := 1; i <= 2; i++ {
		black, _ := store.Record("p1", ip, "h1", false, now.Add(time.Duration(i)*time.Minute), cfg)
		if black {
			t.Fatalf("fail %d must not blacklist yet", i)
		}
	}
	black, _ := store.Record("p1", ip, "h1", false, now.Add(3*time.Minute), cfg)
	if !black {
		t.Fatal("third consecutive failure must blacklist")
	}
	if !store.IsBlacklisted("p1", netip.MustParseAddr(ip)) {
		t.Fatal("IsBlacklisted must be true after blacklist transition")
	}
	if store.IsBlacklisted("p2", netip.MustParseAddr(ip)) {
		t.Fatal("blacklist must not leak across platforms")
	}

	// M-1 passes are not enough.
	for i := 1; i <= 2; i++ {
		_, rehab := store.Record("p1", ip, "h1", true, now.Add(time.Duration(10+i)*time.Minute), cfg)
		if rehab {
			t.Fatalf("pass %d must not rehabilitate yet", i)
		}
		if !store.IsBlacklisted("p1", netip.MustParseAddr(ip)) {
			t.Fatalf("pass %d must keep the blacklist active", i)
		}
	}

	// M-th pass rehabilitates.
	_, rehab := store.Record("p1", ip, "h1", true, now.Add(13*time.Minute), cfg)
	if !rehab {
		t.Fatalf("%d consecutive passes must rehabilitate", cfg.RehabSuccessThreshold)
	}
	if store.IsBlacklisted("p1", netip.MustParseAddr(ip)) {
		t.Fatal("IsBlacklisted must be false after rehab")
	}
	if state, _, fails := store.EntryState("p1", ip); state != string(VerdictNormal) || fails != 0 {
		t.Fatalf("state after rehab = %s fails=%d, want normal/0", state, fails)
	}
}

func TestVerdictStore_OneFailureResetsStreak(t *testing.T) {
	store := NewVerdictStore(nil)
	cfg := platform.VerdictConfig{Enabled: true, Threshold: 2, RehabSuccessThreshold: 2}
	ip := "5.6.7.8"
	now := time.Now()

	store.Record("p1", ip, "h1", false, now, cfg)
	store.Record("p1", ip, "h1", false, now.Add(time.Minute), cfg)
	if !store.IsBlacklisted("p1", netip.MustParseAddr(ip)) {
		t.Fatal("threshold 2 must blacklist on the second failure")
	}

	store.Record("p1", ip, "h1", true, now.Add(2*time.Minute), cfg)
	store.Record("p1", ip, "h1", false, now.Add(3*time.Minute), cfg)
	if state, _, fails := store.EntryState("p1", ip); state != string(VerdictBlacklisted) || fails != 1 {
		t.Fatalf("state=%s fails=%d, want blacklisted/1", state, fails)
	}
	// The failure reset the pass streak, so one pass is still not enough.
	if _, rehab := store.Record("p1", ip, "h1", true, now.Add(4*time.Minute), cfg); rehab {
		t.Fatal("one pass after a failure must not rehabilitate")
	}
}

func TestVerdictStore_NewIPStartsUnknown(t *testing.T) {
	store := NewVerdictStore(nil)
	state, _, fails := store.EntryState("p1", "9.9.9.9")
	if state != "" || fails != 0 {
		t.Fatalf("new IP state = %q fails=%d, want empty/0", state, fails)
	}
	if store.IsBlacklisted("p1", netip.MustParseAddr("9.9.9.9")) {
		t.Fatal("a never-seen IP must not be blacklisted")
	}
}

func TestVerdictStore_DisabledVerdictNeverBlacklists(t *testing.T) {
	store := NewVerdictStore(nil)
	cfg := platform.VerdictConfig{Enabled: false, Threshold: 1}
	for i := 0; i < 5; i++ {
		if black, _ := store.Record("p1", "8.8.8.8", "h1", false, time.Now(), cfg); black {
			t.Fatal("verdict disabled must never blacklist")
		}
	}
}

package platform

import (
	"testing"
	"time"

	"github.com/Resinat/Resin/internal/node"
)

func rebuildOne(t *testing.T, p *Platform, h node.Hash, entry *node.NodeEntry) {
	t.Helper()
	p.FullRebuild(func(fn func(node.Hash, *node.NodeEntry) bool) {
		fn(h, entry)
	}, alwaysLookup, usGeoLookup)
}

// No samples must not filter (post-restart safety). A failure drops the rate to
// zero and filters; a subsequent pass brings it back over the threshold.
func TestFilter_MinProbeSuccessRateAfterProbes(t *testing.T) {
	p := NewPlatform("p-rate", "Rate", nil, nil)
	p.Policy = Policy{MinProbeSuccessRate: 0.5}

	h := makeHash(`{"type":"ss","server":"7.7.7.1","port":443}`)
	entry := makeFullyRoutableEntry(h, "sub1")

	// No samples yet -> dimension does not apply.
	rebuildOne(t, p, h, entry)
	if p.View().Size() != 1 {
		t.Fatalf("no samples must pass, got size %d", p.View().Size())
	}

	// One failure -> 0/1 = 0 < 0.5 -> filtered.
	entry.RecordProbeResult(false, 0, time.Now())
	rebuildOne(t, p, h, entry)
	if p.View().Size() != 0 {
		t.Fatalf("0%% success must be filtered, got size %d", p.View().Size())
	}

	// One pass -> 1/2 = 0.5, not below the threshold -> allowed.
	entry.RecordProbeResult(true, 10*time.Millisecond, time.Now())
	rebuildOne(t, p, h, entry)
	if p.View().Size() != 1 {
		t.Fatalf("50%% success must pass a 0.5 threshold, got size %d", p.View().Size())
	}
}

// A never-probed node is exempt; once probed, the streak must be met.
func TestFilter_MinAliveStreakAfterProbes(t *testing.T) {
	p := NewPlatform("p-streak", "Streak", nil, nil)
	p.Policy = Policy{MinAliveStreak: 2}

	h := makeHash(`{"type":"ss","server":"7.7.7.2","port":443}`)
	entry := makeFullyRoutableEntry(h, "sub1")

	rebuildOne(t, p, h, entry)
	if p.View().Size() != 1 {
		t.Fatalf("never-probed node must be exempt, got size %d", p.View().Size())
	}

	entry.RecordProbeResult(true, 10*time.Millisecond, time.Now())
	rebuildOne(t, p, h, entry)
	if p.View().Size() != 0 {
		t.Fatalf("streak 1 < 2 must be filtered, got size %d", p.View().Size())
	}

	entry.RecordProbeResult(true, 10*time.Millisecond, time.Now())
	rebuildOne(t, p, h, entry)
	if p.View().Size() != 1 {
		t.Fatalf("streak 2 must pass, got size %d", p.View().Size())
	}
}

// A failure resets the streak even after several passes.
func TestFilter_MinAliveStreakResetsOnFailure(t *testing.T) {
	p := NewPlatform("p-streak2", "Streak2", nil, nil)
	p.Policy = Policy{MinAliveStreak: 2}

	h := makeHash(`{"type":"ss","server":"7.7.7.3","port":443}`)
	entry := makeFullyRoutableEntry(h, "sub1")
	entry.RecordProbeResult(true, 10*time.Millisecond, time.Now())
	entry.RecordProbeResult(true, 10*time.Millisecond, time.Now())
	rebuildOne(t, p, h, entry)
	if p.View().Size() != 1 {
		t.Fatal("setup: streak 2 must pass")
	}

	entry.RecordProbeResult(false, 0, time.Now())
	rebuildOne(t, p, h, entry)
	if p.View().Size() != 0 {
		t.Fatal("a failure must reset the streak and filter the node")
	}
}

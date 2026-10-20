package platform

import (
	"net/netip"
	"testing"
	"time"

	"github.com/Resinat/Resin/internal/node"
)

// A platform with no policy configured must keep routing exactly as before:
// every otherwise-routable node stays routable.
func TestPlatform_ZeroPolicyDoesNotFilter(t *testing.T) {
	p := NewPlatform("p-zero", "Zero", nil, nil)
	if !p.Policy.IsZero() {
		t.Fatal("new platform must start with a zero policy")
	}

	h := makeHash(`{"type":"ss","server":"9.9.9.9","port":443}`)
	entry := makeFullyRoutableEntry(h, "sub1")

	p.FullRebuild(func(fn func(node.Hash, *node.NodeEntry) bool) {
		fn(h, entry)
	}, alwaysLookup, usGeoLookup)

	if p.View().Size() != 1 {
		t.Fatalf("zero policy filtered a routable node: size = %d, want 1", p.View().Size())
	}
}

// Explicitly zero-valued policy fields (as loaded from an old row) must behave
// the same as an unset policy.
func TestPlatform_ExplicitZeroPolicyDoesNotFilter(t *testing.T) {
	p := NewPlatform("p-zero2", "Zero2", nil, nil)
	p.Policy = Policy{
		LatencyTopN:         0,
		LatencyMaxMs:        0,
		LatencyP95MaxMs:     0,
		MinAliveStreak:      0,
		MinProbeSuccessRate: 0,
		MaxLastProbeAge:     0,
		MinNodeAge:          0,
		EgressIPVersion:     "",
	}

	h := makeHash(`{"type":"ss","server":"8.8.8.8","port":443}`)
	entry := makeFullyRoutableEntry(h, "sub1")
	entry.SetEgressIP(netip.MustParseAddr("8.8.8.8"))
	entry.LatencyTable.LoadEntry("example.com", node.DomainLatencyStats{Ewma: 20 * time.Millisecond, LastUpdated: time.Now()})

	p.FullRebuild(func(fn func(node.Hash, *node.NodeEntry) bool) {
		fn(h, entry)
	}, alwaysLookup, usGeoLookup)

	if p.View().Size() != 1 {
		t.Fatalf("explicit zero policy filtered a routable node: size = %d, want 1", p.View().Size())
	}
}

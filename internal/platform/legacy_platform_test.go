package platform

import (
	"net/netip"
	"testing"
	"time"

	"github.com/Resinat/Resin/internal/model"
	"github.com/Resinat/Resin/internal/node"
	"github.com/Resinat/Resin/internal/testutil"
)

// legacyPlatform builds the platform a pre-policy version would have persisted:
// only v1..v9 columns set, every new policy/probe/failover field at its zero
// value.
func legacyPlatform() model.Platform {
	return model.Platform{
		ID:                               "legacy-1",
		Name:                             "Legacy",
		StickyTTLNs:                      int64(time.Hour),
		RegexFilters:                     []string{"^agg/"},
		RegionFilters:                    []string{"us"},
		ReverseProxyMissAction:           "TREAT_AS_EMPTY",
		ReverseProxyEmptyAccountBehavior: "RANDOM",
		PassiveCircuitBreakerDisabled:    true,
		AllocationPolicy:                 "PREFER_LOW_LATENCY",
	}
}

func withLookup(subID string) node.SubLookupFunc {
	return func(id string, h node.Hash) (string, bool, []string, bool) {
		if id != subID {
			return "", false, nil, false
		}
		return "agg", true, []string{"us-fast", "jp-slow"}, true
	}
}

func legacyEntry(t *testing.T, raw string, ip string, subID string) (node.Hash, *node.NodeEntry) {
	t.Helper()
	h := makeHash(raw)
	e := node.NewNodeEntry(h, nil, time.Now(), 16)
	e.AddSubscriptionID(subID)
	e.LatencyTable.LoadEntry("example.com", node.DomainLatencyStats{
		Ewma:        100 * time.Millisecond,
		LastUpdated: time.Now(),
	})
	ob := testutil.NewNoopOutbound()
	e.Outbound.Store(&ob)
	e.SetEgressIP(netip.MustParseAddr(ip))
	return h, e
}

// A platform persisted before this change must keep routing exactly as before:
// the zero-valued policy/probe/failover fields must not add or remove a node.
func TestLegacyPlatform_UpgradedBehaviourIsUnchanged(t *testing.T) {
	const subID = "sub-legacy"
	const usIP = "1.2.3.4"
	const jpIP = "5.6.7.8"

	usHash, usEntry := legacyEntry(t, `{"type":"ss","server":"1.2.3.4","port":443}`, usIP, subID)
	jpHash, jpEntry := legacyEntry(t, `{"type":"ss","server":"5.6.7.8","port":443}`, jpIP, subID)

	plat, err := BuildFromModel(legacyPlatform())
	if err != nil {
		t.Fatalf("BuildFromModel: %v", err)
	}
	if !plat.Policy.IsZero() {
		t.Fatal("legacy platform must decode to a zero policy")
	}
	if plat.Probe.Enabled || plat.Verdict.Enabled || plat.Failover.Enabled {
		t.Fatal("legacy platform must decode with probe/verdict/failover disabled")
	}

	usGeo := func(addr netip.Addr) string {
		if addr.String() == usIP {
			return "us"
		}
		return "jp"
	}
	plat.FullRebuild(func(fn func(node.Hash, *node.NodeEntry) bool) {
		fn(usHash, usEntry)
		fn(jpHash, jpEntry)
	}, withLookup(subID), usGeo)

	// US node matches the pre-existing regex + region filters.
	if !plat.View().Contains(usHash) {
		t.Fatal("node matching regex+region must stay routable")
	}
	// JP node is excluded by region_filters exactly as before.
	if plat.View().Contains(jpHash) {
		t.Fatal("node excluded by region_filters must stay excluded")
	}
	if got := plat.View().Size(); got != 1 {
		t.Fatalf("view size = %d, want 1 (unchanged from pre-policy behaviour)", got)
	}
}

// The exclusion must come from the old filters, not from any new dimension:
// with region_filters removed, the JP node becomes routable again.
func TestLegacyPlatform_OnlyOldFiltersApply(t *testing.T) {
	const subID = "sub-legacy2"
	const usIP = "1.2.3.5"
	const jpIP = "5.6.7.9"

	usHash, usEntry := legacyEntry(t, `{"type":"ss","server":"1.2.3.5","port":443}`, usIP, subID)
	jpHash, jpEntry := legacyEntry(t, `{"type":"ss","server":"5.6.7.9","port":443}`, jpIP, subID)

	mp := legacyPlatform()
	mp.RegionFilters = nil
	plat, err := BuildFromModel(mp)
	if err != nil {
		t.Fatalf("BuildFromModel: %v", err)
	}

	plat.FullRebuild(func(fn func(node.Hash, *node.NodeEntry) bool) {
		fn(usHash, usEntry)
		fn(jpHash, jpEntry)
	}, withLookup(subID), func(netip.Addr) string { return "jp" })

	if got := plat.View().Size(); got != 2 {
		t.Fatalf("view size = %d, want 2: with no region filter both old-style nodes must pass", got)
	}
}

package routing

import (
	"net/netip"
	"testing"
	"time"

	"github.com/Resinat/Resin/internal/model"
	"github.com/Resinat/Resin/internal/node"
	"github.com/Resinat/Resin/internal/platform"
	"github.com/Resinat/Resin/internal/subscription"
	"github.com/Resinat/Resin/internal/testutil"
	"github.com/Resinat/Resin/internal/topology"
)

// A platform persisted before the policy work must route exactly as before: with
// a single routable node, RouteRequest must pick it and create a lease. New
// zero-valued fields must not change the outcome.
func TestLegacyPlatform_ZeroNewFieldsRoutesSame(t *testing.T) {
	subMgr := topology.NewSubscriptionManager()
	sub := subscription.NewSubscription("sub-l", "sub-l", "https://example.com/sub", true, false)
	subMgr.Register(sub)

	pool := topology.NewGlobalNodePool(topology.PoolConfig{
		SubLookup:              subMgr.Lookup,
		GeoLookup:              func(netip.Addr) string { return "us" },
		MaxLatencyTableEntries: 16,
		MaxConsecutiveFailures: func() int { return 3 },
		LatencyDecayWindow:     func() time.Duration { return 10 * time.Minute },
	})

	const ip = "10.40.0.1"
	raw := []byte(`{"type":"ss","server":"10.40.0.1","port":443}`)
	h := node.HashFromRawOptions(raw)
	pool.AddNodeFromSub(h, raw, sub.ID)
	sub.ManagedNodes().StoreNode(h, subscription.ManagedNode{Tags: []string{"us-fast"}})
	e, ok := pool.GetEntry(h)
	if !ok {
		t.Fatal("node missing")
	}
	ob := testutil.NewNoopOutbound()
	e.Outbound.Store(&ob)
	e.CircuitOpenSince.Store(0)
	e.SetEgressIP(netip.MustParseAddr(ip))
	e.LatencyTable.LoadEntry("example.com", node.DomainLatencyStats{Ewma: 100 * time.Millisecond, LastUpdated: time.Now()})

	// Exactly what a pre-policy state.db row would decode to.
	mp := model.Platform{
		ID:                               "legacy-r",
		Name:                             "LegacyR",
		StickyTTLNs:                      int64(time.Hour),
		RegexFilters:                     []string{"^sub-l/"},
		RegionFilters:                    []string{"us"},
		ReverseProxyMissAction:           "TREAT_AS_EMPTY",
		ReverseProxyEmptyAccountBehavior: "RANDOM",
		AllocationPolicy:                 "PREFER_LOW_LATENCY",
		PassiveCircuitBreakerDisabled:    true,
	}
	plat, err := platform.BuildFromModel(mp)
	if err != nil {
		t.Fatalf("BuildFromModel: %v", err)
	}
	if !plat.Policy.IsZero() || plat.Probe.Enabled || plat.Verdict.Enabled || plat.Failover.Enabled {
		t.Fatal("legacy platform must decode with all new switches off")
	}

	poolRange := func(fn func(node.Hash, *node.NodeEntry) bool) { pool.Range(fn) }
	plat.FullRebuild(poolRange, pool.MakeSubLookup(), func(netip.Addr) string { return "us" })
	pool.RegisterPlatform(plat)

	router := NewRouter(RouterConfig{
		Pool:        pool,
		Authorities: func() []string { return []string{"example.com"} },
		P2CWindow:   func() time.Duration { return time.Hour },
	})
	res, err := router.RouteRequest("LegacyR", "acct", "https://example.com/x")
	if err != nil {
		t.Fatalf("RouteRequest: %v", err)
	}
	if res.NodeHash != h {
		t.Fatalf("routed to %s, want the only routable node %s", res.NodeHash.Hex(), h.Hex())
	}
	if !res.LeaseCreated {
		t.Fatal("first request must create a sticky lease")
	}
	if lease := router.ReadLease(model.LeaseKey{PlatformID: "legacy-r", Account: "acct"}); lease == nil || lease.NodeHash != h.Hex() {
		t.Fatalf("lease not pinned to the node: %+v", lease)
	}
}

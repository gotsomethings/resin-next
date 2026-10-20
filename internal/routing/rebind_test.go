package routing

import (
	"net/netip"
	"testing"
	"time"

	"github.com/Resinat/Resin/internal/node"
	"github.com/Resinat/Resin/internal/platform"
	"github.com/Resinat/Resin/internal/subscription"
	"github.com/Resinat/Resin/internal/testutil"
	"github.com/Resinat/Resin/internal/topology"
)

type rebindFixture struct {
	pool   *topology.GlobalNodePool
	router *Router
	hA     node.Hash
	hB     node.Hash
	hC     node.Hash
}

func newRebindFixture(t *testing.T, policy platform.AllocationPolicy, latA, latB time.Duration) rebindFixture {
	t.Helper()

	subMgr := topology.NewSubscriptionManager()
	sub := subscription.NewSubscription("sub-1", "sub-1", "https://example.com/sub", true, false)
	subMgr.Register(sub)

	pool := topology.NewGlobalNodePool(topology.PoolConfig{
		SubLookup:              subMgr.Lookup,
		GeoLookup:              func(netip.Addr) string { return "us" },
		MaxLatencyTableEntries: 16,
		MaxConsecutiveFailures: func() int { return 3 },
		LatencyDecayWindow:     func() time.Duration { return 10 * time.Minute },
	})

	mk := func(tag, ip string, latency time.Duration) node.Hash {
		raw := []byte(`{"type":"ss","server":"` + ip + `","port":443}`)
		h := node.HashFromRawOptions(raw)
		pool.AddNodeFromSub(h, raw, sub.ID)
		sub.ManagedNodes().StoreNode(h, subscription.ManagedNode{Tags: []string{tag}})
		e, ok := pool.GetEntry(h)
		if !ok {
			t.Fatalf("node %s missing", tag)
		}
		ob := testutil.NewNoopOutbound()
		e.Outbound.Store(&ob)
		e.CircuitOpenSince.Store(0)
		e.SetEgressIP(netip.MustParseAddr(ip))
		e.LatencyTable.LoadEntry("example.com", node.DomainLatencyStats{Ewma: latency, LastUpdated: time.Now()})
		return h
	}

	hA := mk("a", "10.10.0.1", latA)
	hB := mk("b", "10.10.0.2", latB)
	hC := mk("c", "10.10.0.3", latA)

	plat := platform.NewPlatform("p1", "P1", nil, nil)
	plat.StickyTTLNs = int64(time.Hour)
	plat.AllocationPolicy = policy
	poolRange := func(fn func(node.Hash, *node.NodeEntry) bool) { pool.Range(fn) }
	plat.FullRebuild(poolRange, pool.MakeSubLookup(), func(netip.Addr) string { return "us" })
	pool.RegisterPlatform(plat)

	router := NewRouter(RouterConfig{
		Pool:        pool,
		Authorities: func() []string { return []string{"example.com"} },
		P2CWindow:   func() time.Duration { return time.Hour },
	})

	return rebindFixture{pool: pool, router: router, hA: hA, hB: hB, hC: hC}
}

// PREFER_LOW_LATENCY must pick the faster node, not a random one.
func TestRebindLease_PrefersLowLatency(t *testing.T) {
	fx := newRebindFixture(t, platform.AllocationPolicyPreferLowLatency, 50*time.Millisecond, 500*time.Millisecond)

	// Exclude the third node so only the 50ms/500ms pair remains.
	res, err := fx.router.RebindLease("P1", "acct", fx.hC)
	if err != nil {
		t.Fatalf("RebindLease: %v", err)
	}
	if res.NodeHash != fx.hA {
		t.Fatalf("picked %s, want the 50ms node %s", res.NodeHash.Hex(), fx.hA.Hex())
	}
}

// BALANCED must avoid the IP that already carries leases.
func TestRebindLease_RespectsIPLoadBalancing(t *testing.T) {
	fx := newRebindFixture(t, platform.AllocationPolicyBalanced, 100*time.Millisecond, 100*time.Millisecond)

	state := fx.router.ensurePlatformState("p1")
	for i := 0; i < 3; i++ {
		state.IPLoadStats.Inc(netip.MustParseAddr("10.10.0.1"))
	}

	// Exclude the third node; candidates are A (loaded) and B (idle).
	res, err := fx.router.RebindLease("P1", "acct", fx.hC)
	if err != nil {
		t.Fatalf("RebindLease: %v", err)
	}
	if res.NodeHash != fx.hB {
		t.Fatalf("picked %s, want the idle-IP node %s", res.NodeHash.Hex(), fx.hB.Hex())
	}
}

// The failed node must never come back, no matter how often we rebind.
func TestRebindLease_ExcludesFailedNode(t *testing.T) {
	fx := newRebindFixture(t, platform.AllocationPolicyBalanced, 100*time.Millisecond, 100*time.Millisecond)

	for i := 0; i < 50; i++ {
		res, err := fx.router.RebindLease("P1", "acct", fx.hA)
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if res.NodeHash == fx.hA {
			t.Fatalf("iteration %d: returned the failed node %s", i, fx.hA.Hex())
		}
	}
}

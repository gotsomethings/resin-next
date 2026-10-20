package service

import (
	"net/netip"
	"testing"
	"time"

	"github.com/Resinat/Resin/internal/model"
	"github.com/Resinat/Resin/internal/node"
	"github.com/Resinat/Resin/internal/platform"
	"github.com/Resinat/Resin/internal/routing"
	"github.com/Resinat/Resin/internal/subscription"
	"github.com/Resinat/Resin/internal/testutil"
	"github.com/Resinat/Resin/internal/topology"
)

// evictFixture builds a platform with two routable nodes on different egress IPs.
func evictFixture(t *testing.T) (*ControlPlaneService, *routing.Router, node.Hash, node.Hash) {
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

	mk := func(tag, ip string) node.Hash {
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
		e.LatencyTable.LoadEntry("example.com", node.DomainLatencyStats{Ewma: 50 * time.Millisecond, LastUpdated: time.Now()})
		return h
	}

	hA := mk("a", "10.1.0.1")
	hB := mk("b", "10.1.0.2")

	plat := platform.NewPlatform("plat-evict", "Evict", nil, nil)
	plat.StickyTTLNs = int64(time.Hour)
	poolRange := func(fn func(node.Hash, *node.NodeEntry) bool) { pool.Range(fn) }
	plat.FullRebuild(poolRange, pool.MakeSubLookup(), func(netip.Addr) string { return "us" })
	pool.RegisterPlatform(plat)

	router := routing.NewRouter(routing.RouterConfig{Pool: pool})
	cp := &ControlPlaneService{Pool: pool, Router: router, SubMgr: subMgr}
	return cp, router, hA, hB
}

func pinLease(t *testing.T, router *routing.Router, account string, h node.Hash, ip string) {
	t.Helper()
	now := time.Now()
	if err := router.UpsertLease(model.Lease{
		PlatformID:     "plat-evict",
		Account:        account,
		NodeHash:       h.Hex(),
		EgressIP:       ip,
		CreatedAtNs:    now.UnixNano(),
		ExpiryNs:       now.Add(time.Hour).UnixNano(),
		LastAccessedNs: now.UnixNano(),
	}); err != nil {
		t.Fatalf("upsert lease %s: %v", account, err)
	}
}

// Blacklisting an IP must drop exactly the leases pinned to it, and leave the
// per-IP load counters consistent.
func TestEvictLeasesForIP_RemovesOnlyMatchingLeases(t *testing.T) {
	cp, router, hA, hB := evictFixture(t)
	pinLease(t, router, "acct-a", hA, "10.1.0.1")
	pinLease(t, router, "acct-b", hB, "10.1.0.2")

	before := router.SnapshotIPLoad("plat-evict")
	if before[netip.MustParseAddr("10.1.0.1")] != 1 || before[netip.MustParseAddr("10.1.0.2")] != 1 {
		t.Fatalf("setup IP load wrong: %v", before)
	}

	n := cp.EvictLeasesForIP("plat-evict", "10.1.0.1")
	if n != 1 {
		t.Fatalf("evicted %d leases, want 1", n)
	}

	if l := router.ReadLease(model.LeaseKey{PlatformID: "plat-evict", Account: "acct-a"}); l != nil {
		t.Fatalf("lease on the blacklisted IP must be gone, got %+v", l)
	}
	if l := router.ReadLease(model.LeaseKey{PlatformID: "plat-evict", Account: "acct-b"}); l == nil {
		t.Fatal("lease on the other IP must survive")
	}

	after := router.SnapshotIPLoad("plat-evict")
	if after[netip.MustParseAddr("10.1.0.1")] != 0 {
		t.Fatalf("IP A load = %d, want 0", after[netip.MustParseAddr("10.1.0.1")])
	}
	if after[netip.MustParseAddr("10.1.0.2")] != 1 {
		t.Fatalf("IP B load = %d, want 1", after[netip.MustParseAddr("10.1.0.2")])
	}
}

// Evicting an IP nobody is pinned to is a no-op.
func TestEvictLeasesForIP_NoMatchIsNoop(t *testing.T) {
	cp, router, hA, _ := evictFixture(t)
	pinLease(t, router, "acct-a", hA, "10.1.0.1")

	if n := cp.EvictLeasesForIP("plat-evict", "10.9.9.9"); n != 0 {
		t.Fatalf("evicted %d, want 0", n)
	}
	if l := router.ReadLease(model.LeaseKey{PlatformID: "plat-evict", Account: "acct-a"}); l == nil {
		t.Fatal("unrelated lease must survive")
	}
}

// Multiple accounts on the same IP are all dropped.
func TestEvictLeasesForIP_DropsEveryAccountOnThatIP(t *testing.T) {
	cp, router, hA, hB := evictFixture(t)
	pinLease(t, router, "acct-1", hA, "10.1.0.1")
	pinLease(t, router, "acct-2", hA, "10.1.0.1")
	pinLease(t, router, "acct-3", hB, "10.1.0.2")

	if n := cp.EvictLeasesForIP("plat-evict", "10.1.0.1"); n != 2 {
		t.Fatalf("evicted %d, want 2", n)
	}
	for _, acct := range []string{"acct-1", "acct-2"} {
		if l := router.ReadLease(model.LeaseKey{PlatformID: "plat-evict", Account: acct}); l != nil {
			t.Fatalf("%s must be evicted", acct)
		}
	}
	if l := router.ReadLease(model.LeaseKey{PlatformID: "plat-evict", Account: "acct-3"}); l == nil {
		t.Fatal("acct-3 on the other IP must survive")
	}
}

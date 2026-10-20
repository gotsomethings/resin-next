package proxy

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/Resinat/Resin/internal/model"
	"github.com/Resinat/Resin/internal/node"
	"github.com/Resinat/Resin/internal/platform"
	"github.com/Resinat/Resin/internal/routing"
	"github.com/Resinat/Resin/internal/subscription"
	"github.com/Resinat/Resin/internal/topology"
	"github.com/sagernet/sing-box/adapter"
	M "github.com/sagernet/sing/common/metadata"
)

func failoverFixture(t *testing.T, failA, failB bool) (*topology.GlobalNodePool, *routing.Router, node.Hash, node.Hash) {
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

	makeNode := func(tag, ip string, fail bool) node.Hash {
		raw := []byte(`{"type":"ss","server":"` + ip + `","port":443}`)
		h := node.HashFromRawOptions(raw)
		pool.AddNodeFromSub(h, raw, sub.ID)
		sub.ManagedNodes().StoreNode(h, subscription.ManagedNode{Tags: []string{tag}})
		entry, ok := pool.GetEntry(h)
		if !ok {
			t.Fatalf("node %s missing", tag)
		}
		var ob adapter.Outbound
		if fail {
			ob = &mockOutbound{dialFunc: func(context.Context, string, M.Socksaddr) (net.Conn, error) {
				return nil, errors.New("dial refused")
			}}
		} else {
			ob = &mockOutbound{dialFunc: func(context.Context, string, M.Socksaddr) (net.Conn, error) {
				client, server := net.Pipe()
				go func() { _, _ = server.Read(make([]byte, 1)); _ = server.Close() }()
				return client, nil
			}}
		}
		entry.Outbound.Store(&ob)
		entry.CircuitOpenSince.Store(0)
		entry.SetEgressIP(netip.MustParseAddr(ip))
		entry.LatencyTable.LoadEntry("example.com", node.DomainLatencyStats{Ewma: 50 * time.Millisecond, LastUpdated: time.Now()})
		return h
	}

	hA := makeNode("a", "10.0.0.1", failA)
	hB := makeNode("b", "10.0.0.2", failB)

	plat := platform.NewPlatform("p1", "P1", nil, nil)
	plat.StickyTTLNs = int64(time.Hour)
	plat.Failover = platform.FailoverConfig{Enabled: true, MaxSwitches: 2, Deadline: 5 * time.Second, Stage: "connect"}
	poolRange := func(fn func(node.Hash, *node.NodeEntry) bool) { pool.Range(fn) }
	plat.FullRebuild(poolRange, pool.MakeSubLookup(), func(netip.Addr) string { return "us" })
	pool.RegisterPlatform(plat)

	router := routing.NewRouter(routing.RouterConfig{Pool: pool})
	return pool, router, hA, hB
}

func leaseOn(t *testing.T, router *routing.Router, h node.Hash, ip string) {
	t.Helper()
	now := time.Now()
	if err := router.UpsertLease(model.Lease{
		PlatformID:     "p1",
		Account:        "acct",
		NodeHash:       h.Hex(),
		EgressIP:       ip,
		CreatedAtNs:    now.UnixNano(),
		ExpiryNs:       now.Add(time.Hour).UnixNano(),
		LastAccessedNs: now.UnixNano(),
	}); err != nil {
		t.Fatalf("upsert lease: %v", err)
	}
}

func TestConnectFailover_RebindsLeaseToNewNode(t *testing.T) {
	pool, router, hA, hB := failoverFixture(t, true, false)
	leaseOn(t, router, hA, "10.0.0.1")

	res := prepareConnectTunnel(context.Background(), tunnelDeps{router: router, pool: pool}, "P1", "acct", "example.com:443")
	if res.session == nil {
		t.Fatalf("expected failover session, got proxyErr=%v stage=%s err=%v", res.proxyErr, res.upstreamStage, res.upstreamErr)
	}
	defer res.session.upstreamConn.Close()

	if res.route.NodeHash != hB {
		t.Fatalf("failover route = %s, want %s", res.route.NodeHash.Hex(), hB.Hex())
	}
	lease := router.ReadLease(model.LeaseKey{PlatformID: "p1", Account: "acct"})
	if lease == nil || lease.NodeHash != hB.Hex() {
		t.Fatalf("lease must be re-pinned to %s, got %+v", hB.Hex(), lease)
	}
}

func TestConnectFailover_BudgetExhaustedReleasesLease(t *testing.T) {
	pool, router, hA, _ := failoverFixture(t, true, true)
	leaseOn(t, router, hA, "10.0.0.1")

	res := prepareConnectTunnel(context.Background(), tunnelDeps{router: router, pool: pool}, "P1", "acct", "example.com:443")
	if res.session != nil {
		t.Fatal("all nodes failed: session must be nil")
	}
	if res.proxyErr == nil {
		t.Fatal("all nodes failed: original proxy error must be surfaced")
	}
	if res.upstreamStage != "connect_dial" {
		t.Fatalf("stage = %q, want connect_dial", res.upstreamStage)
	}
	if lease := router.ReadLease(model.LeaseKey{PlatformID: "p1", Account: "acct"}); lease != nil {
		t.Fatalf("all nodes failed: lease must be released, got %+v", lease)
	}
}

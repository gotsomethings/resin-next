package service

import (
	"net/netip"
	"path/filepath"
	"testing"
	"time"

	"github.com/Resinat/Resin/internal/model"
	"github.com/Resinat/Resin/internal/node"
	"github.com/Resinat/Resin/internal/platform"
	"github.com/Resinat/Resin/internal/state"
	"github.com/Resinat/Resin/internal/subscription"
	"github.com/Resinat/Resin/internal/testutil"
	"github.com/Resinat/Resin/internal/topology"
)

// The platform IP list is the platform's IP library: a circuit-open node is
// already unusable for Resin, so its IP must not be listed even though it still
// matches the static platform filters.
func TestListPlatformEgress_ExcludesCircuitOpenNodes(t *testing.T) {
	dir := t.TempDir()
	engine, closer, err := state.PersistenceBootstrap(filepath.Join(dir, "state"), filepath.Join(dir, "cache"))
	if err != nil {
		t.Fatalf("PersistenceBootstrap: %v", err)
	}
	t.Cleanup(func() { _ = closer.Close() })

	row := model.Platform{
		ID:                     platform.DefaultPlatformID,
		Name:                   platform.DefaultPlatformName,
		StickyTTLNs:            int64(2 * time.Hour),
		ReverseProxyMissAction: string(platform.ReverseProxyMissActionTreatAsEmpty),
		AllocationPolicy:       string(platform.AllocationPolicyBalanced),
		UpdatedAtNs:            time.Now().UnixNano(),
	}
	if err := engine.UpsertPlatform(row); err != nil {
		t.Fatalf("UpsertPlatform: %v", err)
	}

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
	plat := platform.NewConfiguredPlatform(
		row.ID,
		row.Name,
		node.TagFilter{},
		nil,
		row.StickyTTLNs,
		row.ReverseProxyMissAction,
		string(platform.ReverseProxyEmptyAccountBehaviorAccountHeaderRule),
		"",
		row.AllocationPolicy,
		true,
	)
	pool.RegisterPlatform(plat)

	addNode := func(server, egressIP string, circuitOpen bool) {
		t.Helper()
		raw := []byte(`{"type":"ss","server":"` + server + `","port":443}`)
		h := node.HashFromRawOptions(raw)
		pool.AddNodeFromSub(h, raw, sub.ID)
		sub.ManagedNodes().StoreNode(h, subscription.ManagedNode{Tags: []string{"all"}})
		entry, ok := pool.GetEntry(h)
		if !ok {
			t.Fatalf("entry %s missing", server)
		}
		ob := testutil.NewNoopOutbound()
		entry.Outbound.Store(&ob)
		entry.SetEgressIP(netip.MustParseAddr(egressIP))
		entry.LatencyTable.LoadEntry("example.com", node.DomainLatencyStats{Ewma: 100 * time.Millisecond, LastUpdated: time.Now()})
		if circuitOpen {
			entry.CircuitOpenSince.Store(time.Now().UnixNano())
			return
		}
		entry.CircuitOpenSince.Store(0)
	}

	addNode("1.1.1.1", "1.1.1.1", false)
	addNode("2.2.2.2", "2.2.2.2", true)

	// The list is served from the platform's routable view, so build it the same
	// way the runtime does after nodes change.
	pool.RebuildPlatform(plat)

	cp := &ControlPlaneService{Engine: engine, Pool: pool, SubMgr: subMgr}
	rows, err := cp.ListPlatformEgress(platform.DefaultPlatformID)
	if err != nil {
		t.Fatalf("ListPlatformEgress: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows: got %d, want 1 (circuit-open node must be excluded): %+v", len(rows), rows)
	}
	if rows[0].IP != "1.1.1.1" {
		t.Fatalf("ip: got %q, want 1.1.1.1", rows[0].IP)
	}
	if rows[0].NodeCount != 1 {
		t.Fatalf("node_count: got %d, want 1", rows[0].NodeCount)
	}

	// A blacklisted IP stays listed while its node is still healthy: that row is
	// how an operator sees the state and releases it again.
	plat.Verdict.Enabled = true
	plat.BlacklistLookup = func(ip netip.Addr) bool { return ip.String() == "1.1.1.1" }
	rows, err = cp.ListPlatformEgress(platform.DefaultPlatformID)
	if err != nil {
		t.Fatalf("ListPlatformEgress (blacklisted): %v", err)
	}
	if len(rows) != 1 || rows[0].IP != "1.1.1.1" {
		t.Fatalf("blacklisted IP must stay listed while healthy: %+v", rows)
	}
}

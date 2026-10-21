package service

import (
	"net/netip"
	"testing"
	"time"

	"github.com/Resinat/Resin/internal/geoip"
	"github.com/Resinat/Resin/internal/subscription"
	"github.com/Resinat/Resin/internal/topology"
)

// stubASNReader maps one egress IP to AS13335 / Cloudflare and everything else
// to "not in database".
type stubASNReader struct{}

func (stubASNReader) LookupASN(ip netip.Addr) (geoip.ASNInfo, bool) {
	if ip.String() == "203.0.113.10" {
		return geoip.ASNInfo{Number: 13335, Org: "Cloudflare, Inc."}, true
	}
	return geoip.ASNInfo{}, false
}

func (stubASNReader) Close() error { return nil }

func newASNTestService(t *testing.T) (*ControlPlaneService, *topology.GlobalNodePool, *subscription.Subscription) {
	t.Helper()
	subMgr := topology.NewSubscriptionManager()
	pool := newNodeListTestPool(subMgr)
	sub := subscription.NewSubscription("sub-asn", "sub-asn", "https://example.com/asn", true, false)
	subMgr.Register(sub)
	cp := &ControlPlaneService{
		Pool:   pool,
		SubMgr: subMgr,
		GeoIP:  &geoip.Service{},
		ASN:    geoip.NewASNServiceWithReader(stubASNReader{}),
	}
	return cp, pool, sub
}

// The ASN column is filled from the ASN service, and asn= filters on both the
// number and the organization name.
func TestListNodes_ASNColumnAndFilter(t *testing.T) {
	cp, pool, sub := newASNTestService(t)

	hash := addRoutableNodeForSubscription(t, pool, sub,
		[]byte(`{"type":"ss","server":"1.1.1.1","port":443}`), "203.0.113.10")
	addRoutableNodeForSubscription(t, pool, sub,
		[]byte(`{"type":"ss","server":"2.2.2.2","port":443}`), "203.0.113.11")

	nodes, err := cp.ListNodes(NodeFilters{})
	if err != nil {
		t.Fatalf("ListNodes: %v", err)
	}
	var found bool
	for _, n := range nodes {
		if n.NodeHash != hash.Hex() {
			continue
		}
		found = true
		if n.ASNNumber != 13335 || n.ASNOrg != "Cloudflare, Inc." {
			t.Fatalf("ASN = %d/%q, want 13335/Cloudflare, Inc.", n.ASNNumber, n.ASNOrg)
		}
	}
	if !found {
		t.Fatalf("probed node %s missing from the list", hash.Hex())
	}

	byNumber := "13335"
	hits, err := cp.ListNodes(NodeFilters{ASN: &byNumber})
	if err != nil {
		t.Fatalf("ListNodes(asn=13335): %v", err)
	}
	if len(hits) != 1 || hits[0].NodeHash != hash.Hex() {
		t.Fatalf("asn=13335 returned %d nodes, want the Cloudflare one", len(hits))
	}

	byOrg := "cloudflare"
	hits, err = cp.ListNodes(NodeFilters{ASN: &byOrg})
	if err != nil {
		t.Fatalf("ListNodes(asn=cloudflare): %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("asn=cloudflare returned %d nodes, want 1", len(hits))
	}

	miss := "64500"
	hits, err = cp.ListNodes(NodeFilters{ASN: &miss})
	if err != nil {
		t.Fatalf("ListNodes(asn=64500): %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("asn=64500 returned %d nodes, want 0", len(hits))
	}
}

// pending_test keeps only nodes that were never probed.
func TestListNodes_PendingTestFilter(t *testing.T) {
	cp, pool, sub := newASNTestService(t)

	probed := addRoutableNodeForSubscription(t, pool, sub,
		[]byte(`{"type":"ss","server":"3.3.3.3","port":443}`), "203.0.113.10")
	pending := addRoutableNodeForSubscription(t, pool, sub,
		[]byte(`{"type":"ss","server":"4.4.4.4","port":443}`), "203.0.113.11")

	probedEntry, _ := pool.GetEntry(probed)
	probedEntry.LastProbeAttempt.Store(time.Now().UnixNano())

	pendingEntry, _ := pool.GetEntry(pending)
	pendingEntry.SetEgressIP(netip.Addr{})
	pendingEntry.LastProbeAttempt.Store(0)

	yes := true
	hits, err := cp.ListNodes(NodeFilters{PendingTest: &yes})
	if err != nil {
		t.Fatalf("ListNodes(pending_test=true): %v", err)
	}
	if len(hits) != 1 || hits[0].NodeHash != pending.Hex() {
		t.Fatalf("pending_test=true returned %d nodes, want only %s", len(hits), pending.Hex())
	}
	if !hits[0].PendingTest {
		t.Fatal("pending node must report pending_test=true")
	}

	no := false
	hits, err = cp.ListNodes(NodeFilters{PendingTest: &no})
	if err != nil {
		t.Fatalf("ListNodes(pending_test=false): %v", err)
	}
	if len(hits) != 1 || hits[0].NodeHash != probed.Hex() {
		t.Fatalf("pending_test=false returned %d nodes, want only %s", len(hits), probed.Hex())
	}
}

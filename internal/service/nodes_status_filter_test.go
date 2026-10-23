package service

import (
	"testing"
	"time"
)

// The two switches are independent: each alone filters by its own field, and
// both together mean union (OR) — a node that is both must appear once.
func TestListNodes_CircuitOpenPendingTestUnion(t *testing.T) {
	cp, pool, sub := newASNTestService(t)

	circuitOnly := addRoutableNodeForSubscription(t, pool, sub,
		[]byte(`{"type":"ss","server":"5.5.5.5","port":443}`), "203.0.113.10")
	pendingOnly := addRoutableNodeForSubscription(t, pool, sub,
		[]byte(`{"type":"ss","server":"6.6.6.6","port":443}`), "203.0.113.11")
	both := addRoutableNodeForSubscription(t, pool, sub,
		[]byte(`{"type":"ss","server":"7.7.7.7","port":443}`), "203.0.113.12")

	// circuitOnly: probed, breaker tripped.
	e, _ := pool.GetEntry(circuitOnly)
	e.LastProbeAttempt.Store(time.Now().UnixNano())
	e.CircuitOpenSince.Store(time.Now().UnixNano())

	// pendingOnly: never probed, breaker closed.
	e, _ = pool.GetEntry(pendingOnly)
	e.LastProbeAttempt.Store(0)

	// both: breaker tripped and never probed.
	e, _ = pool.GetEntry(both)
	e.LastProbeAttempt.Store(0)
	e.CircuitOpenSince.Store(time.Now().UnixNano())

	yes := true

	hits, err := cp.ListNodes(NodeFilters{CircuitOpen: &yes})
	if err != nil {
		t.Fatalf("circuit_open=true: %v", err)
	}
	if got := hashSet(hits); len(got) != 2 || !got[circuitOnly.Hex()] || !got[both.Hex()] {
		t.Fatalf("circuit_open=true returned %v, want circuitOnly+both", got)
	}

	hits, err = cp.ListNodes(NodeFilters{PendingTest: &yes})
	if err != nil {
		t.Fatalf("pending_test=true: %v", err)
	}
	if got := hashSet(hits); len(got) != 2 || !got[pendingOnly.Hex()] || !got[both.Hex()] {
		t.Fatalf("pending_test=true returned %v, want pendingOnly+both", got)
	}

	hits, err = cp.ListNodes(NodeFilters{CircuitOpen: &yes, PendingTest: &yes})
	if err != nil {
		t.Fatalf("both switches: %v", err)
	}
	got := hashSet(hits)
	if len(got) != 3 || !got[circuitOnly.Hex()] || !got[pendingOnly.Hex()] || !got[both.Hex()] {
		t.Fatalf("both switches returned %v, want the union of three", got)
	}

	// Other filters still AND: only the ASN-matching circuit node survives.
	asn := "13335"
	hits, err = cp.ListNodes(NodeFilters{CircuitOpen: &yes, PendingTest: &yes, ASN: &asn})
	if err != nil {
		t.Fatalf("both switches + asn: %v", err)
	}
	got = hashSet(hits)
	if len(got) != 1 || !got[circuitOnly.Hex()] {
		t.Fatalf("both + asn=13335 returned %v, want only circuitOnly", got)
	}
}

// An off switch must not filter: pending_test=false alone keeps probed nodes.
func TestListNodes_SwitchesOffDoNotFilter(t *testing.T) {
	cp, pool, sub := newASNTestService(t)

	h := addRoutableNodeForSubscription(t, pool, sub,
		[]byte(`{"type":"ss","server":"8.8.8.8","port":443}`), "203.0.113.10")
	e, _ := pool.GetEntry(h)
	e.LastProbeAttempt.Store(time.Now().UnixNano())

	no := false
	hits, err := cp.ListNodes(NodeFilters{CircuitOpen: &no, PendingTest: &no})
	if err != nil {
		t.Fatalf("ListNodes: %v", err)
	}
	if len(hits) != 1 || hits[0].NodeHash != h.Hex() {
		t.Fatalf("switches off returned %d nodes, want the single healthy one", len(hits))
	}
}

func hashSet(nodes []NodeSummary) map[string]bool {
	set := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		set[n.NodeHash] = true
	}
	return set
}

// The UI is single-choice: picking one status must never return the other
// class, while a node that is both shows up under either pick.
func TestListNodes_CircuitOpenPendingTestAreExclusive(t *testing.T) {
	cp, pool, sub := newASNTestService(t)

	circuitOnly := addRoutableNodeForSubscription(t, pool, sub,
		[]byte(`{"type":"ss","server":"9.9.9.9","port":443}`), "203.0.113.20")
	pendingOnly := addRoutableNodeForSubscription(t, pool, sub,
		[]byte(`{"type":"ss","server":"9.9.9.10","port":443}`), "203.0.113.21")
	both := addRoutableNodeForSubscription(t, pool, sub,
		[]byte(`{"type":"ss","server":"9.9.9.11","port":443}`), "203.0.113.22")

	e, _ := pool.GetEntry(circuitOnly)
	e.LastProbeAttempt.Store(time.Now().UnixNano())
	e.CircuitOpenSince.Store(time.Now().UnixNano())

	e, _ = pool.GetEntry(pendingOnly)
	e.LastProbeAttempt.Store(0)

	e, _ = pool.GetEntry(both)
	e.LastProbeAttempt.Store(0)
	e.CircuitOpenSince.Store(time.Now().UnixNano())

	yes := true

	circuitHits, err := cp.ListNodes(NodeFilters{CircuitOpen: &yes})
	if err != nil {
		t.Fatalf("circuit_open=true: %v", err)
	}
	circuitSet := hashSet(circuitHits)
	if circuitSet[pendingOnly.Hex()] {
		t.Fatal("a purely pending node must not match circuit_open=true")
	}
	if !circuitSet[circuitOnly.Hex()] || !circuitSet[both.Hex()] {
		t.Fatalf("circuit_open=true missing expected nodes: %v", circuitSet)
	}

	pendingHits, err := cp.ListNodes(NodeFilters{PendingTest: &yes})
	if err != nil {
		t.Fatalf("pending_test=true: %v", err)
	}
	pendingSet := hashSet(pendingHits)
	if pendingSet[circuitOnly.Hex()] {
		t.Fatal("a purely circuit-open node must not match pending_test=true")
	}
	if !pendingSet[pendingOnly.Hex()] || !pendingSet[both.Hex()] {
		t.Fatalf("pending_test=true missing expected nodes: %v", pendingSet)
	}
}

// The status dropdown must return exactly the nodes whose badge shows that
// status: 健康 excludes never-probed nodes, 待测 excludes tripped breakers.
func TestListNodes_StatusBucketsMatchBadge(t *testing.T) {
	cp, pool, sub := newASNTestService(t)

	healthy := addRoutableNodeForSubscription(t, pool, sub,
		[]byte(`{"type":"ss","server":"10.10.10.1","port":443}`), "203.0.113.10")
	pending := addRoutableNodeForSubscription(t, pool, sub,
		[]byte(`{"type":"ss","server":"10.10.10.2","port":443}`), "203.0.113.11")
	tripped := addRoutableNodeForSubscription(t, pool, sub,
		[]byte(`{"type":"ss","server":"10.10.10.3","port":443}`), "203.0.113.12")

	e, _ := pool.GetEntry(healthy)
	e.LastProbeAttempt.Store(time.Now().UnixNano())

	e, _ = pool.GetEntry(pending)
	e.LastProbeAttempt.Store(0)

	e, _ = pool.GetEntry(tripped)
	e.LastProbeAttempt.Store(0)
	e.CircuitOpenSince.Store(time.Now().UnixNano())

	yes, no := true, false

	// 健康: enabled && has outbound && !circuit_open && !pending_test
	hits, err := cp.ListNodes(NodeFilters{CircuitOpen: &no, PendingTest: &no})
	if err != nil {
		t.Fatalf("健康: %v", err)
	}
	if got := hashSet(hits); len(got) != 1 || !got[healthy.Hex()] {
		t.Fatalf("健康 returned %v, want only the healthy node", got)
	}

	// 待测: pending_test && !circuit_open
	hits, err = cp.ListNodes(NodeFilters{CircuitOpen: &no, PendingTest: &yes})
	if err != nil {
		t.Fatalf("待测: %v", err)
	}
	if got := hashSet(hits); len(got) != 1 || !got[pending.Hex()] {
		t.Fatalf("待测 returned %v, want only the pending node", got)
	}

	// 熔断: circuit_open
	hits, err = cp.ListNodes(NodeFilters{CircuitOpen: &yes})
	if err != nil {
		t.Fatalf("熔断: %v", err)
	}
	if got := hashSet(hits); len(got) != 1 || !got[tripped.Hex()] {
		t.Fatalf("熔断 returned %v, want only the tripped node", got)
	}
}

// ip_type filters on the class derived from the ASN org.
func TestListNodes_IPTypeFilter(t *testing.T) {
	cp, pool, sub := newASNTestService(t)

	residential := addRoutableNodeForSubscription(t, pool, sub,
		[]byte(`{"type":"ss","server":"10.10.10.4","port":443}`), "203.0.113.41")
	hosting := addRoutableNodeForSubscription(t, pool, sub,
		[]byte(`{"type":"ss","server":"10.10.10.5","port":443}`), "203.0.113.40")
	unknown := addRoutableNodeForSubscription(t, pool, sub,
		[]byte(`{"type":"ss","server":"10.10.10.6","port":443}`), "203.0.113.99")

	res := "residential"
	hits, err := cp.ListNodes(NodeFilters{IPType: &res})
	if err != nil {
		t.Fatalf("ip_type=residential: %v", err)
	}
	if got := hashSet(hits); len(got) != 1 || !got[residential.Hex()] {
		t.Fatalf("ip_type=residential returned %v", got)
	}

	host := "hosting"
	hits, err = cp.ListNodes(NodeFilters{IPType: &host})
	if err != nil {
		t.Fatalf("ip_type=hosting: %v", err)
	}
	got := hashSet(hits)
	if len(got) != 1 || !got[hosting.Hex()] {
		t.Fatalf("ip_type=hosting returned %v", got)
	}
	if got[unknown.Hex()] {
		t.Fatal("a node without ASN data must not match a concrete ip_type")
	}

	unk := "unknown"
	hits, err = cp.ListNodes(NodeFilters{IPType: &unk})
	if err != nil {
		t.Fatalf("ip_type=unknown: %v", err)
	}
	if got := hashSet(hits); len(got) != 1 || !got[unknown.Hex()] {
		t.Fatalf("ip_type=unknown returned %v, want only the node with no ASN data", got)
	}
}

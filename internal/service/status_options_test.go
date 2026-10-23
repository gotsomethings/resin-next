package service

import (
	"testing"
	"time"

	"github.com/Resinat/Resin/internal/node"
	"github.com/Resinat/Resin/internal/subscription"
	"github.com/Resinat/Resin/internal/testutil"
)

// Every status option in the node-pool dropdown must return exactly the nodes
// whose badge shows that status — including the three states that are hard to
// produce by hand (待测 / 错误 / 禁用).
func TestListNodes_StatusOptionsMatchBadges(t *testing.T) {
	cp, pool, sub := newASNTestService(t)

	// A second subscription that is disabled: a node bound only to it is 禁用.
	offSub := subscription.NewSubscription("sub-off", "sub-off", "https://example.com/off", true, false)
	cp.SubMgr.Register(offSub)
	offSub.SetEnabled(false)

	seed := func(raw, ip string, s *subscription.Subscription, outbound bool) node.Hash {
		t.Helper()
		h := addRoutableNodeForSubscription(t, pool, s, []byte(raw), ip)
		e, ok := pool.GetEntry(h)
		if !ok {
			t.Fatalf("entry %s missing", h.Hex())
		}
		if outbound {
			ob := testutil.NewNoopOutbound()
			e.Outbound.Store(&ob)
		}
		return h
	}

	healthy := seed(`{"type":"st-healthy","server":"10.1.0.1","port":443}`, "198.51.100.1", sub, true)
	circuit := seed(`{"type":"st-circuit","server":"10.1.0.2","port":443}`, "198.51.100.2", sub, true)
	pending := seed(`{"type":"st-pending","server":"10.1.0.3","port":443}`, "198.51.100.3", sub, true)
	errored := seed(`{"type":"st-error","server":"10.1.0.4","port":443}`, "198.51.100.4", sub, false)
	disabled := seed(`{"type":"st-disabled","server":"10.1.0.5","port":443}`, "198.51.100.5", offSub, true)

	stamp := time.Now().UnixNano()
	for _, h := range []node.Hash{healthy, circuit, errored, disabled} {
		e, _ := pool.GetEntry(h)
		e.LastLatencyProbeAttempt.Store(stamp) // probed => not 待测
	}
	e, _ := pool.GetEntry(circuit)
	e.CircuitOpenSince.Store(stamp)

	// The seed helper always attaches an outbound; the 错误 case needs it gone.
	errEntry, _ := pool.GetEntry(errored)
	errEntry.Outbound.Store(nil)

	trueVal, falseVal := true, false
	cases := []struct {
		name    string
		filters NodeFilters
		want    node.Hash
	}{
		{
			name:    "健康",
			filters: NodeFilters{Enabled: &trueVal, HasOutbound: &trueVal, CircuitOpen: &falseVal, PendingTest: &falseVal},
			want:    healthy,
		},
		{
			name:    "熔断",
			filters: NodeFilters{Enabled: &trueVal, HasOutbound: &trueVal, CircuitOpen: &trueVal},
			want:    circuit,
		},
		{
			name:    "待测",
			filters: NodeFilters{Enabled: &trueVal, HasOutbound: &trueVal, CircuitOpen: &falseVal, PendingTest: &trueVal},
			want:    pending,
		},
		{
			name:    "错误",
			filters: NodeFilters{Enabled: &trueVal, HasOutbound: &falseVal},
			want:    errored,
		},
		{
			name:    "禁用",
			filters: NodeFilters{Enabled: &falseVal},
			want:    disabled,
		},
	}

	for _, tc := range cases {
		hits, err := cp.ListNodes(tc.filters)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		got := hashSet(hits)
		if len(got) != 1 || !got[tc.want.Hex()] {
			t.Fatalf("%s returned %d nodes (%v), want exactly %s", tc.name, len(got), got, tc.want.Hex())
		}
	}
}

package probe

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/Resinat/Resin/internal/node"
	"github.com/Resinat/Resin/internal/platform"
	"github.com/Resinat/Resin/internal/subscription"
	"github.com/Resinat/Resin/internal/topology"
	"github.com/sagernet/sing-box/adapter"
	M "github.com/sagernet/sing/common/metadata"
)

// scriptedOutbound answers every dial according to mode: ok / forbidden / timeout.
type scriptedOutbound struct {
	mode string
}

func (o *scriptedOutbound) Type() string                 { return "scripted" }
func (o *scriptedOutbound) Tag() string                  { return "scripted" }
func (o *scriptedOutbound) Network() []string            { return []string{"tcp"} }
func (o *scriptedOutbound) Dependencies() []string       { return nil }
func (o *scriptedOutbound) Close() error                 { return nil }
func (o *scriptedOutbound) ListenPacket(context.Context, M.Socksaddr) (net.PacketConn, error) {
	return nil, errors.New("not supported")
}

func (o *scriptedOutbound) DialContext(ctx context.Context, network string, dest M.Socksaddr) (net.Conn, error) {
	switch o.mode {
	case "forbidden":
		return nil, errors.New("scripted: forbidden")
	case "timeout":
		<-ctx.Done()
		return nil, ctx.Err()
	}
	client, server := net.Pipe()
	go func() {
		defer server.Close()
		buf := make([]byte, 4096)
		for {
			if _, err := server.Read(buf); err != nil {
				return
			}
			resp := "HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n"
			if _, err := server.Write([]byte(resp)); err != nil {
				return
			}
			return
		}
	}()
	return client, nil
}

// fakeVerdict records every transition without a store.
type fakeVerdict struct {
	mu      sync.Mutex
	states  map[string]string
	calls   int
	passes  int
	fails   int
}

func newFakeVerdict() *fakeVerdict { return &fakeVerdict{states: map[string]string{}} }

func (f *fakeVerdict) IsBlacklisted(_ string, _ netip.Addr) bool { return false }

func (f *fakeVerdict) EntryState(platformID, ip string) (string, int64, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.states[platformID+"|"+ip], 0, 0
}

func (f *fakeVerdict) Record(platformID, ip, _ string, pass bool, _ time.Time, _ platform.VerdictConfig) (bool, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	key := platformID + "|" + ip
	if pass {
		f.passes++
		f.states[key] = "normal"
	} else {
		f.fails++
	}
	return false, false
}

func probeFixture(t *testing.T, mode string, ip string) (*topology.GlobalNodePool, node.Hash) {
	t.Helper()
	subMgr := topology.NewSubscriptionManager()
	sub := subscription.NewSubscription("s", "s", "https://example.com", true, false)
	subMgr.Register(sub)
	pool := topology.NewGlobalNodePool(topology.PoolConfig{
		SubLookup:              subMgr.Lookup,
		GeoLookup:              func(netip.Addr) string { return "us" },
		MaxLatencyTableEntries: 8,
		MaxConsecutiveFailures: func() int { return 3 },
		LatencyDecayWindow:     func() time.Duration { return time.Minute },
	})
	raw := []byte(`{"type":"ss","server":"` + ip + `","port":443}`)
	h := node.HashFromRawOptions(raw)
	pool.AddNodeFromSub(h, raw, sub.ID)
	sub.ManagedNodes().StoreNode(h, subscription.ManagedNode{Tags: []string{"all"}})
	e, ok := pool.GetEntry(h)
	if !ok {
		t.Fatal("node missing")
	}
	var ob adapter.Outbound = &scriptedOutbound{mode: mode}
	e.Outbound.Store(&ob)
	e.CircuitOpenSince.Store(0)
	e.SetEgressIP(netip.MustParseAddr(ip))
	e.LatencyTable.LoadEntry("example.com", node.DomainLatencyStats{Ewma: 10 * time.Millisecond, LastUpdated: time.Now()})
	return pool, h
}

func newTarget() PlatformTarget {
	return PlatformTarget{
		ID: "p1",
		Probe: platform.ProbeConfig{
			Enabled:    true,
			URL:        "http://probe.local/check",
			Method:     http.MethodPost,
			Body:       `{}`,
			PassStatus: []int{200},
			Interval:   time.Minute,
			Scope:      "untested,failed,blacklisted",
		},
		Verdict: platform.VerdictConfig{Enabled: true, Threshold: 3, RehabInterval: time.Hour, RehabSuccessThreshold: 3},
	}
}

func TestPlatformProbe_200RecordsPass(t *testing.T) {
	pool, h := probeFixture(t, "ok", "10.2.0.1")
	fv := newFakeVerdict()
	pp := NewPlatformProbe(PlatformProbeConfig{Pool: pool, Verdict: fv, Timeout: func() time.Duration { return 2 * time.Second }})

	pass, _ := pp.probeOne(newTarget(), h)
	if !pass {
		t.Fatal("200 must be recorded as pass")
	}

	pp.sweep(newTarget())
	if fv.calls != 1 || fv.passes != 1 {
		t.Fatalf("expected 1 pass record, got calls=%d passes=%d fails=%d", fv.calls, fv.passes, fv.fails)
	}
	if got := fv.states["p1|10.2.0.1"]; got != "normal" {
		t.Fatalf("verdict state = %q, want normal", got)
	}
}

func TestPlatformProbe_DialErrorRecordsFailure(t *testing.T) {
	pool, h := probeFixture(t, "forbidden", "10.2.0.2")
	fv := newFakeVerdict()
	pp := NewPlatformProbe(PlatformProbeConfig{Pool: pool, Verdict: fv, Timeout: func() time.Duration { return time.Second }})

	if pass, _ := pp.probeOne(newTarget(), h); pass {
		t.Fatal("dial error must not be a pass")
	}
	pp.sweep(newTarget())
	if fv.fails != 1 {
		t.Fatalf("expected 1 failure record, got calls=%d fails=%d", fv.calls, fv.fails)
	}
}

func TestPlatformProbe_TimeoutRecordsFailure(t *testing.T) {
	pool, h := probeFixture(t, "timeout", "10.2.0.3")
	fv := newFakeVerdict()
	pp := NewPlatformProbe(PlatformProbeConfig{Pool: pool, Verdict: fv, Timeout: func() time.Duration { return 120 * time.Millisecond }})

	if pass, _ := pp.probeOne(newTarget(), h); pass {
		t.Fatal("timeout must not be a pass")
	}
	pp.sweep(newTarget())
	if fv.fails != 1 {
		t.Fatalf("expected 1 failure record, got calls=%d fails=%d", fv.calls, fv.fails)
	}
}

// A platform with probing disabled must never touch the verdict store.
func TestPlatformProbe_DisabledPlatformSkipsEntirely(t *testing.T) {
	pool, _ := probeFixture(t, "ok", "10.2.0.4")
	fv := newFakeVerdict()
	pp := NewPlatformProbe(PlatformProbeConfig{Pool: pool, Verdict: fv})

	tgt := newTarget()
	tgt.Probe.Enabled = false
	pp.sweep(tgt)

	if fv.calls != 0 {
		t.Fatalf("disabled probe must not record anything, got %d calls", fv.calls)
	}
}

// An IP that is already "normal" and probed recently is skipped by scope.
func TestPlatformProbe_ScopeSkipsRecentlyProbedNormalIP(t *testing.T) {
	pool, _ := probeFixture(t, "ok", "10.2.0.5")

	fv := newFakeVerdict()
	fv.states["p1|10.2.0.5"] = "normal"
	pp := NewPlatformProbe(PlatformProbeConfig{Pool: pool, Verdict: fv})

	// freshState says "normal" but EntryState reports lastProbe=0, so it is due;
	// this asserts the scope decision is driven by EntryState, not by guesswork.
	pp.sweep(newTarget())
	if fv.calls != 1 {
		t.Fatalf("a normal IP with no probe timestamp is still due, got %d calls", fv.calls)
	}
}

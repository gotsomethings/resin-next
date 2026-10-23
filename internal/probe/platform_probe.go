package probe

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/Resinat/Resin/internal/node"
	"github.com/Resinat/Resin/internal/platform"
	"github.com/Resinat/Resin/internal/topology"
	M "github.com/sagernet/sing/common/metadata"
)

// VerdictRecorder is implemented by service.VerdictStore.
type VerdictRecorder interface {
	IsBlacklisted(platformID string, ip netip.Addr) bool
	EntryState(platformID, ip string) (state string, lastProbeAtNs int64, failureCount int)
	Record(platformID, ip, nodeHash string, pass bool, statusCode int, now time.Time, cfg platform.VerdictConfig) (bool, bool)
}

// PlatformTarget is a snapshot of one platform's probe/verdict config.
type PlatformTarget struct {
	ID      string
	Probe   platform.ProbeConfig
	Verdict platform.VerdictConfig
}

// PlatformProbeConfig configures the per-platform probe loop.
type PlatformProbeConfig struct {
	Pool            *topology.GlobalNodePool
	Targets         func() []PlatformTarget
	Verdict         VerdictRecorder
	Timeout         func() time.Duration
	Concurrency     int
	TickInterval    time.Duration
	MaxPerSweep     int
	OnVerdictChange func(platformID string)
}

// PlatformProbe probes each platform's egress IPs with that platform's own
// request template and records pass/fail into the verdict store.
type PlatformProbe struct {
	cfg      PlatformProbeConfig
	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// NewPlatformProbe creates a PlatformProbe.
func NewPlatformProbe(cfg PlatformProbeConfig) *PlatformProbe {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 8
	}
	if cfg.TickInterval <= 0 {
		cfg.TickInterval = 30 * time.Second
	}
	if cfg.MaxPerSweep <= 0 {
		cfg.MaxPerSweep = 500
	}
	return &PlatformProbe{cfg: cfg, stopCh: make(chan struct{})}
}

// Start launches the background sweep loop.
func (p *PlatformProbe) Start() {
	if p == nil {
		return
	}
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		ticker := time.NewTicker(p.cfg.TickInterval)
		defer ticker.Stop()
		p.sweepAll()
		for {
			select {
			case <-p.stopCh:
				return
			case <-ticker.C:
				p.sweepAll()
			}
		}
	}()
}

// Stop stops the loop and waits for in-flight sweeps.
func (p *PlatformProbe) Stop() {
	if p == nil {
		return
	}
	p.stopOnce.Do(func() { close(p.stopCh) })
	p.wg.Wait()
}

func (p *PlatformProbe) sweepAll() {
	if p.cfg.Targets == nil || p.cfg.Verdict == nil || p.cfg.Pool == nil {
		return
	}
	for _, t := range p.cfg.Targets() {
		if !t.Probe.Enabled || t.Probe.URL == "" {
			continue
		}
		select {
		case <-p.stopCh:
			return
		default:
		}
		p.sweep(t)
	}
}

type probeCandidate struct {
	hash node.Hash
	ip   netip.Addr
}

// sweep probes one platform. Disabled platforms are a hard no-op: the guard
// lives here (not only in sweepAll) so every entry point honours the switch.
func (p *PlatformProbe) sweep(t PlatformTarget) {
	if !t.Probe.Enabled || t.Probe.URL == "" {
		return
	}
	now := time.Now()
	interval := t.Probe.Interval
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	rehab := t.Verdict.RehabInterval
	if rehab <= 0 {
		rehab = 6 * time.Hour
	}

	byIP := make(map[string]probeCandidate)
	p.cfg.Pool.Range(func(h node.Hash, entry *node.NodeEntry) bool {
		ip := entry.GetEgressIP()
		if !ip.IsValid() || !entry.IsHealthy() {
			return true
		}
		key := ip.String()
		if _, exists := byIP[key]; exists {
			return true
		}

		state, lastProbe, _ := p.cfg.Verdict.EntryState(t.ID, key)
		due := false
		switch state {
		case "":
			due = true
		case "blacklisted":
			// 黑名单复用关闭时，拉黑 IP 不再重探。
			if t.Verdict.RehabEnabled {
				due = lastProbe == 0 || now.Sub(time.Unix(0, lastProbe)) >= rehab
			}
		default:
			due = lastProbe == 0 || now.Sub(time.Unix(0, lastProbe)) >= interval
		}
		if !due {
			return true
		}
		byIP[key] = probeCandidate{hash: h, ip: ip.Unmap()}
		return true
	})

	if len(byIP) == 0 {
		return
	}

	sem := make(chan struct{}, p.cfg.Concurrency)
	var wg sync.WaitGroup
	count := 0
	for _, c := range byIP {
		if count >= p.cfg.MaxPerSweep {
			break
		}
		count++
		wg.Add(1)
		sem <- struct{}{}
		go func(c probeCandidate) {
			defer wg.Done()
			defer func() { <-sem }()
			pass, latency, statusCode := p.probeOne(t, c.hash)
			now := time.Now()
			// The probe belongs to the EGRESS IP, not to one node: every node behind
			// that IP shares the result, so node-level filters stay meaningful.
			p.recordIPProbeResult(c.ip, pass, latency, now)
			black, rehabbed := p.cfg.Verdict.Record(t.ID, c.ip.String(), c.hash.Hex(), pass, statusCode, now, t.Verdict)
			if (black || rehabbed) && p.cfg.OnVerdictChange != nil {
				p.cfg.OnVerdictChange(t.ID)
			}
		}(c)
	}
	wg.Wait()
}

func (p *PlatformProbe) probeOne(t PlatformTarget, hash node.Hash) (bool, time.Duration, int) {
	entry, ok := p.cfg.Pool.GetEntry(hash)
	if !ok {
		return false, 0, 0
	}
	outboundPtr := entry.Outbound.Load()
	if outboundPtr == nil {
		return false, 0, 0
	}

	timeout := 15 * time.Second
	if p.cfg.Timeout != nil {
		if d := p.cfg.Timeout(); d > 0 {
			timeout = d
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	method := strings.ToUpper(strings.TrimSpace(t.Probe.Method))
	if method == "" {
		method = http.MethodPost
	}
	var body io.Reader
	if t.Probe.Body != "" {
		body = strings.NewReader(t.Probe.Body)
	}
	req, err := http.NewRequestWithContext(ctx, method, t.Probe.URL, body)
	if err != nil {
		return false, 0, 0
	}
	for _, line := range strings.Split(t.Probe.Headers, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		req.Header.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
	}

	dialer := func(ctx context.Context, network, addr string) (net.Conn, error) {
		return (*outboundPtr).DialContext(ctx, network, M.ParseSocksaddr(addr))
	}
	tr := &http.Transport{DialContext: dialer}
	client := &http.Client{Transport: tr, Timeout: timeout}
	// Probing never follows redirects: whatever the target returns is the
	// result, so a 3xx is reported as-is instead of being chased.
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)
	if err != nil {
		return false, latency, 0
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	passStatus := t.Probe.PassStatus
	if len(passStatus) == 0 {
		passStatus = []int{http.StatusOK}
	}
	for _, code := range passStatus {
		if resp.StatusCode == code {
			return true, latency, resp.StatusCode
		}
	}
	return false, latency, resp.StatusCode
}

// recordIPProbeResult writes one probe outcome into every node sharing the IP.
// latency is only used on pass; failures record zero so P95 stays honest.
func (p *PlatformProbe) recordIPProbeResult(ip netip.Addr, pass bool, latency time.Duration, now time.Time) {
	lat := time.Duration(0)
	if pass {
		lat = latency
		if lat <= 0 {
			// A successful probe always has some latency; a zero reading is a
			// clock-resolution artifact. Record 1ns so the sample is counted.
			lat = time.Nanosecond
		}
	}
	p.cfg.Pool.Range(func(h node.Hash, entry *node.NodeEntry) bool {
		if entry.GetEgressIP().Unmap() != ip {
			return true
		}
		entry.RecordProbeResult(pass, lat, now)
		return true
	})
}

package platform

import (
	"net/netip"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/Resinat/Resin/internal/node"
)

// DefaultPlatformID is the well-known UUID of the built-in Default platform.
const DefaultPlatformID = "00000000-0000-0000-0000-000000000000"

// DefaultPlatformName is the built-in platform name.
const DefaultPlatformName = "Default"

// GeoLookupFunc resolves an IP address to a lowercase ISO country code.
type GeoLookupFunc func(netip.Addr) string

// PoolRangeFunc iterates all nodes in the global pool.
type PoolRangeFunc func(fn func(node.Hash, *node.NodeEntry) bool)

// GetEntryFunc retrieves a node entry from the global pool by hash.
type GetEntryFunc func(node.Hash) (*node.NodeEntry, bool)

// Policy holds the 11-dimension filter surface added in P1.
// Zero values mean "not configured" and never filter anything out.
type Policy struct {
	LatencyTopN          int
	LatencyMaxMs         int
	LatencyP95MaxMs      int
	MinAliveStreak       int
	MinProbeSuccessRate  float64 // 0..1
	// MinProbeSuccessRateWindow is the window the rate is computed over;
	// 0 means the fixed 24h view.
	MinProbeSuccessRateWindow time.Duration
	MaxLastProbeAge      time.Duration
	MinNodeAge           time.Duration
	EgressIPVersion      string // "", "v4", "v6"
	EgressCountryFilters []string
	ASNFilters           []string
	IPTypeFilters        []string
	EgressIPFilters      []string
}

// IsZero reports whether no policy dimension is configured.
func (p Policy) IsZero() bool {
	return p.LatencyTopN == 0 && p.LatencyMaxMs == 0 && p.LatencyP95MaxMs == 0 &&
		p.MinAliveStreak == 0 && p.MinProbeSuccessRate == 0 && p.MinProbeSuccessRateWindow == 0 && p.MaxLastProbeAge == 0 &&
		p.MinNodeAge == 0 && p.EgressIPVersion == "" && len(p.EgressCountryFilters) == 0 &&
		len(p.ASNFilters) == 0 && len(p.IPTypeFilters) == 0 && len(p.EgressIPFilters) == 0
}

// Validate checks all policy fields without touching the pool.
func Validate(p Policy) error {
	if p.LatencyTopN < 0 {
		return errorf("latency_top_n: must be >= 0")
	}
	if p.LatencyMaxMs < 0 {
		return errorf("latency_max_ms: must be >= 0")
	}
	if p.LatencyP95MaxMs < 0 {
		return errorf("latency_p95_max_ms: must be >= 0")
	}
	if p.MinAliveStreak < 0 {
		return errorf("min_alive_streak: must be >= 0")
	}
	if p.MinProbeSuccessRate < 0 || p.MinProbeSuccessRate > 1 {
		return errorf("min_probe_success_rate: must be between 0 and 1")
	}
	if p.MinProbeSuccessRateWindow > 0 && p.MinProbeSuccessRateWindow < 10*time.Minute {
		return errorf("probe_rate_window: must be >= 10m")
	}
	if p.MaxLastProbeAge < 0 {
		return errorf("max_last_probe_age: must be >= 0")
	}
	if p.MinNodeAge < 0 {
		return errorf("min_node_age: must be >= 0")
	}
	if err := ValidateEgressIPVersion(p.EgressIPVersion); err != nil {
		return err
	}
	if err := ValidateEgressCountryFilters(p.EgressCountryFilters); err != nil {
		return err
	}
	if err := ValidateASNFilters(p.ASNFilters); err != nil {
		return err
	}
	if err := ValidateIPTypeFilters(p.IPTypeFilters); err != nil {
		return err
	}
	// EgressIPFilters (manual IP allow/deny) was removed as a feature; the field is
	// kept for schema compatibility only and is no longer validated or applied.
	return nil
}

func errorf(msg string) error { return &policyError{msg: msg} }

type policyError struct{ msg string }

func (e *policyError) Error() string { return e.msg }

// ProbeConfig is the per-platform probe definition (P2).
type ProbeConfig struct {
	Enabled         bool
	URL             string
	Method          string
	Headers         string
	Body            string
	PassStatus      []int
	FollowRedirects bool
	Interval        time.Duration
	Scope           string
}

// VerdictConfig is the per-platform verdict/blacklist policy (P2).
type VerdictConfig struct {
	Enabled bool
	// RehabEnabled re-probes blacklisted IPs after RehabInterval.
	RehabEnabled          bool
	Threshold             int
	RehabInterval         time.Duration
	RehabSuccessThreshold int
}

// FailoverConfig controls connect-stage node switching (P4).
type FailoverConfig struct {
	Enabled     bool
	MaxSwitches int
	Deadline    time.Duration
	Stage       string // only "connect" is implemented
}

// Platform represents a routing platform with its filtered routable view.
type Platform struct {
	ID   string
	Name string

	// Filter configuration.
	RegexFilters  node.TagFilter
	RegionFilters []string // lowercase ISO codes, supports negation "!xx"

	// Policy is the 11-dimension filter surface.
	Policy Policy
	// ASNLookup is optional; nil means "no ASN database loaded".
	ASNLookup ASNLookupFunc

	// Probe/Verdict configuration and the blacklist lookup (P2).
	Probe           ProbeConfig
	Verdict         VerdictConfig
	Failover        FailoverConfig
	BlacklistLookup func(netip.Addr) bool

	// Other config fields.
	StickyTTLNs                      int64
	ReverseProxyMissAction           string
	ReverseProxyEmptyAccountBehavior string
	ReverseProxyFixedAccountHeader   string
	ReverseProxyFixedAccountHeaders  []string
	AllocationPolicy                 AllocationPolicy
	PassiveCircuitBreakerDisabled    bool

	// Routable view & its lock.
	// viewMu serializes both FullRebuild and NotifyDirty.
	view   *RoutableView
	viewMu sync.Mutex
}

// NewPlatform creates a Platform with an empty routable view.
// The regex slice is treated as MUST rules for compatibility with internal callers.
func NewPlatform(id, name string, regexFilters []*regexp.Regexp, regionFilters []string) *Platform {
	return NewPlatformWithTagFilter(id, name, node.TagFilter{Must: regexFilters}, regionFilters)
}

// NewPlatformWithTagFilter creates a Platform with compiled line-oriented tag rules.
func NewPlatformWithTagFilter(id, name string, regexFilters node.TagFilter, regionFilters []string) *Platform {
	return &Platform{
		ID:            id,
		Name:          name,
		RegexFilters:  regexFilters,
		RegionFilters: regionFilters,
		view:          NewRoutableView(),
	}
}

// View returns the platform's routable view as a read-only interface.
// External callers cannot Add/Remove/Clear — only FullRebuild and NotifyDirty can mutate.
func (p *Platform) View() ReadOnlyView {
	return p.view
}

// NeedsFullRebuild reports whether incremental NotifyDirty is insufficient.
// A latency Top-N policy is a global ranking, so it always needs a full pass.
func (p *Platform) NeedsFullRebuild() bool {
	return p.Policy.LatencyTopN > 0
}

// FullRebuild clears the routable view and re-evaluates all nodes from the pool.
// Acquires viewMu — any concurrent NotifyDirty calls block until rebuild completes.
func (p *Platform) FullRebuild(
	poolRange PoolRangeFunc,
	subLookup node.SubLookupFunc,
	geoLookup GeoLookupFunc,
) {
	p.viewMu.Lock()
	defer p.viewMu.Unlock()

	p.view.Clear()

	type candidate struct {
		hash    node.Hash
		latency float64
	}
	var candidates []candidate

	poolRange(func(h node.Hash, entry *node.NodeEntry) bool {
		if p.evaluateNode(entry, subLookup, geoLookup) {
			lat, _, ok := nodeLatencyMs(entry)
			if !ok {
				lat = 0
			}
			candidates = append(candidates, candidate{hash: h, latency: lat})
		}
		return true
	})

	if topN := p.Policy.LatencyTopN; topN > 0 && len(candidates) > topN {
		sort.SliceStable(candidates, func(i, j int) bool {
			return candidates[i].latency < candidates[j].latency
		})
		candidates = candidates[:topN]
	}

	for _, c := range candidates {
		p.view.Add(c.hash)
	}
}

// NotifyDirty re-evaluates a single node and adds/removes it from the view.
// Acquires viewMu — serialized with FullRebuild.
// When the platform has a Top-N policy, incremental evaluation is skipped; the
// caller must run FullRebuild (see NeedsFullRebuild).
func (p *Platform) NotifyDirty(
	h node.Hash,
	getEntry GetEntryFunc,
	subLookup node.SubLookupFunc,
	geoLookup GeoLookupFunc,
) {
	p.viewMu.Lock()
	defer p.viewMu.Unlock()

	if p.Policy.LatencyTopN > 0 {
		return
	}

	entry, ok := getEntry(h)
	if !ok {
		// Node was deleted from pool.
		p.view.Remove(h)
		return
	}

	if p.evaluateNode(entry, subLookup, geoLookup) {
		p.view.Add(h)
	} else {
		p.view.Remove(h)
	}
}

// MatchesConfigFilters reports whether the node passes the platform's static
// configuration filters: subscription enablement, tag rules, a known egress IP,
// region, node age, egress IP version, egress country, ASN, IP type and the
// manual egress-IP allow/deny list.
//
// Runtime dimensions (health, latency, alive streak, probe success rate, last
// probe age) and the verdict blacklist are deliberately NOT part of this
// predicate: the IP list still has to show untested and blacklisted IPs.
func (p *Platform) MatchesConfigFilters(
	entry *node.NodeEntry,
	subLookup node.SubLookupFunc,
	geoLookup GeoLookupFunc,
) bool {
	// Disabled nodes are never routable.
	if entry.IsDisabledBySubscriptions(subLookup) {
		return false
	}

	// Tag regex match.
	if !entry.MatchTagFilter(p.RegexFilters, subLookup) {
		return false
	}

	// Egress IP must be known.
	egressIP := entry.GetEgressIP()
	if !egressIP.IsValid() {
		return false
	}

	// Region filter (when configured).
	if len(p.RegionFilters) > 0 {
		region := entry.GetRegion(geoLookup)
		if !MatchRegionFilter(region, p.RegionFilters) {
			return false
		}
	}

	pol := p.Policy
	now := time.Now()

	// min_node_age.
	if pol.MinNodeAge > 0 {
		if now.Sub(entry.CreatedAt) < pol.MinNodeAge {
			return false
		}
	}

	// egress_ip_version.
	if !MatchEgressIPVersion(egressIP, pol.EgressIPVersion) {
		return false
	}

	// egress_country_filters (real egress country, not the tag country).
	if len(pol.EgressCountryFilters) > 0 {
		country := ""
		if geoLookup != nil {
			country = geoLookup(egressIP)
		}
		if !MatchRegionFilter(country, pol.EgressCountryFilters) {
			return false
		}
	}

	// ASN and IP type. Without an ASN database these dimensions cannot be
	// evaluated; treating every node as a mismatch would empty the platform, so
	// they simply do not apply.
	if p.ASNLookup != nil && (len(pol.ASNFilters) > 0 || len(pol.IPTypeFilters) > 0) {
		info, ok := p.ASNLookup(egressIP)
		if !MatchASNFilter(info, ok, pol.ASNFilters) {
			return false
		}
		if !MatchIPTypeFilter(info, ok, pol.IPTypeFilters) {
			return false
		}
	}

	// Manual egress IP allow/deny was removed; the stored column is kept for schema
	// compatibility but no longer participates in matching.

	return true
}

// evaluateNode checks all filter conditions for platform routability.
func (p *Platform) evaluateNode(
	entry *node.NodeEntry,
	subLookup node.SubLookupFunc,
	geoLookup GeoLookupFunc,
) bool {
	return p.evaluateNodeWith(entry, subLookup, geoLookup, true)
}

// EvaluateForList reports whether the node belongs to the platform's IP list.
// It applies the same dimensions as evaluateNode except the verdict blacklist: a
// blacklisted IP stays listed with its blacklist state while Resin still regards
// its nodes as healthy, so a rehab probe or a manual release can revive it
// without the row ever disappearing in between.
func (p *Platform) EvaluateForList(
	entry *node.NodeEntry,
	subLookup node.SubLookupFunc,
	geoLookup GeoLookupFunc,
) bool {
	return p.evaluateNodeWith(entry, subLookup, geoLookup, false)
}

func (p *Platform) evaluateNodeWith(
	entry *node.NodeEntry,
	subLookup node.SubLookupFunc,
	geoLookup GeoLookupFunc,
	useBlacklist bool,
) bool {
	if !p.MatchesConfigFilters(entry, subLookup, geoLookup) {
		return false
	}

	// 1. Healthy for routing (outbound ready + circuit not open).
	if !entry.IsHealthy() {
		return false
	}

	// 2. Per-platform blacklist (verdict state machine). Skipped for the IP list,
	// which reports blacklisted IPs with their state instead of hiding them.
	egressIP := entry.GetEgressIP()
	if useBlacklist && p.Verdict.Enabled && p.BlacklistLookup != nil && p.BlacklistLookup(egressIP) {
		return false
	}

	// 3. Has at least one latency record.
	if !entry.HasLatency() {
		return false
	}

	pol := p.Policy
	now := time.Now()

	// 6. latency_max_ms.
	if pol.LatencyMaxMs > 0 {
		latMs, _, ok := nodeLatencyMs(entry)
		if !ok || latMs > float64(pol.LatencyMaxMs) {
			return false
		}
	}

	// 7. latency_p95_max_ms.
	if pol.LatencyP95MaxMs > 0 {
		// No samples (e.g. right after a restart) => the dimension does not apply.
		p95 := time.Duration(entry.P95LatencyNs.Load())
		if p95 > 0 && p95.Milliseconds() > int64(pol.LatencyP95MaxMs) {
			return false
		}
	}

	// 8. min_alive_streak.
	if pol.MinAliveStreak > 0 && entry.LastProbeAttempt.Load() > 0 {
		// Never-probed nodes are exempt: the streak is in-memory and empty after a
		// restart, so filtering them would empty the pool until the first sweep.
		if int(entry.AliveStreak.Load()) < pol.MinAliveStreak {
			return false
		}
	}

	// 9. min_probe_success_rate.
	if pol.MinProbeSuccessRate > 0 {
		// No samples => the dimension does not apply (avoids a post-restart empty pool).
		if rate, ok := entry.ProbeSuccessRateWindow(now, pol.MinProbeSuccessRateWindow); ok && rate < pol.MinProbeSuccessRate {
			return false
		}
	}

	// 10. max_last_probe_age.
	if pol.MaxLastProbeAge > 0 && entry.LastProbeAttempt.Load() > 0 {
		// Never-probed nodes are exempt for the same reason as min_alive_streak.
		last := entry.LastProbeSuccess.Load()
		if last <= 0 || now.Sub(time.Unix(0, last)) > pol.MaxLastProbeAge {
			return false
		}
	}

	// 11. min_node_age.
	if pol.MinNodeAge > 0 {
		if now.Sub(entry.CreatedAt) < pol.MinNodeAge {
			return false
		}
	}

	// 12. egress_ip_version.
	if !MatchEgressIPVersion(egressIP, pol.EgressIPVersion) {
		return false
	}

	// 13. egress_country_filters (real egress country, not the tag country).
	if len(pol.EgressCountryFilters) > 0 {
		country := ""
		if geoLookup != nil {
			country = geoLookup(egressIP)
		}
		if !MatchRegionFilter(country, pol.EgressCountryFilters) {
			return false
		}
	}

	// 14/15. ASN and IP type.
	// Without an ASN database these dimensions cannot be evaluated; treating every
	// node as a mismatch would empty the platform, so they simply do not apply.
	if p.ASNLookup != nil && (len(pol.ASNFilters) > 0 || len(pol.IPTypeFilters) > 0) {
		info, ok := p.ASNLookup(egressIP)
		if !MatchASNFilter(info, ok, pol.ASNFilters) {
			return false
		}
		if !MatchIPTypeFilter(info, ok, pol.IPTypeFilters) {
			return false
		}
	}

	// 16. Manual egress IP allow/deny was removed; it no longer filters nodes.

	return true
}

// nodeLatencyMs returns the node's average EWMA latency in ms across all
// domains currently in its latency table.
func nodeLatencyMs(entry *node.NodeEntry) (avg float64, p95 float64, ok bool) {
	if entry == nil || entry.LatencyTable == nil {
		return 0, 0, false
	}
	var sum float64
	var count int
	var values []float64
	entry.LatencyTable.Range(func(_ string, stats node.DomainLatencyStats) bool {
		ms := float64(stats.Ewma) / float64(time.Millisecond)
		sum += ms
		count++
		values = append(values, ms)
		return true
	})
	if count == 0 {
		return 0, 0, false
	}
	avg = sum / float64(count)
	sort.Float64s(values)
	idx := int(float64(len(values)-1) * 0.95)
	if idx < 0 {
		idx = 0
	}
	p95 = values[idx]
	return avg, p95, true
}

// MatchRegionFilter applies include/exclude region filters.
// Positive entries (xx) build an include set; negative entries (!xx) build an exclude set.
// Unknown regions never match when region filters are configured.
// Final result is: region known AND (include empty OR region in include) AND (region not in exclude).
func MatchRegionFilter(region string, filters []string) bool {
	if len(filters) == 0 {
		return true
	}
	if region == "" {
		return false
	}

	included := false
	hasInclude := false

	for _, filter := range filters {
		if len(filter) > 0 && filter[0] == '!' {
			if region == filter[1:] {
				return false
			}
			continue
		}
		hasInclude = true
		if region == filter {
			included = true
		}
	}

	if hasInclude && !included {
		return false
	}
	return true
}

package service

import (
	"net/netip"
	"sync"
	"time"

	"github.com/Resinat/Resin/internal/model"
	"github.com/Resinat/Resin/internal/platform"
	"github.com/Resinat/Resin/internal/state"
)

// VerdictStore keeps the per-platform, per-egress-IP verdict state in memory
// and mirrors it to state.db's platform_egress_ips table.
type VerdictStore struct {
	mu         sync.RWMutex
	byPlatform map[string]map[string]*model.PlatformEgressIP
	repo       *state.StateRepo

	onBlacklist func(platformID, ip string)
	onRehab     func(platformID, ip string)
}

// NewVerdictStore creates an empty store backed by repo (repo may be nil).
func NewVerdictStore(repo *state.StateRepo) *VerdictStore {
	return &VerdictStore{
		byPlatform: make(map[string]map[string]*model.PlatformEgressIP),
		repo:       repo,
	}
}

// SetCallbacks installs transition hooks. Call before Start.
func (v *VerdictStore) SetCallbacks(onBlacklist, onRehab func(platformID, ip string)) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.onBlacklist = onBlacklist
	v.onRehab = onRehab
}

// Load hydrates the store from state.db.
func (v *VerdictStore) Load(platformIDs []string) error {
	if v == nil || v.repo == nil {
		return nil
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	for _, pid := range platformIDs {
		rows, err := v.repo.ListPlatformEgressIPs(pid)
		if err != nil {
			return err
		}
		m := make(map[string]*model.PlatformEgressIP, len(rows))
		for i := range rows {
			r := rows[i]
			m[r.IP] = &r
		}
		v.byPlatform[pid] = m
	}
	return nil
}

// EntryState returns the persisted state for an IP.
func (v *VerdictStore) EntryState(platformID, ip string) (state string, lastProbeAtNs int64, failureCount int) {
	if v == nil {
		return "", 0, 0
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	if m, ok := v.byPlatform[platformID]; ok {
		if e, ok := m[ip]; ok {
			return e.State, e.LastProbeAtNs, e.FailureCount
		}
	}
	return "", 0, 0
}

// IsBlacklisted reports whether ip is blacklisted for this platform.
func (v *VerdictStore) IsBlacklisted(platformID string, ip netip.Addr) bool {
	if v == nil || !ip.IsValid() {
		return false
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	if m, ok := v.byPlatform[platformID]; ok {
		if e, ok := m[ip.String()]; ok {
			return e.State == string(VerdictBlacklisted)
		}
	}
	return false
}

// VerdictState values.
type VerdictState string

const (
	VerdictUnknown     VerdictState = "unknown"
	VerdictNormal      VerdictState = "normal"
	VerdictBlacklisted VerdictState = "blacklisted"
)

// Record applies one probe outcome and returns transition flags.
func (v *VerdictStore) Record(
	platformID, ip, nodeHash string,
	pass bool,
	now time.Time,
	cfg platform.VerdictConfig,
) (becameBlacklisted, becameNormal bool) {
	if v == nil {
		return false, false
	}

	v.mu.Lock()
	m := v.byPlatform[platformID]
	if m == nil {
		m = make(map[string]*model.PlatformEgressIP)
		v.byPlatform[platformID] = m
	}
	e := m[ip]
	if e == nil {
		e = &model.PlatformEgressIP{PlatformID: platformID, IP: ip, State: string(VerdictUnknown), FirstSeenNs: now.UnixNano()}
		m[ip] = e
	}

	e.LastProbeAtNs = now.UnixNano()
	e.LastSeenNs = now.UnixNano()
	e.LastNodeHash = nodeHash

	if pass {
		e.LastResult = "pass"
		e.LastSuccessAtNs = now.UnixNano()
		e.SuccessStreak++
		e.FailureCount = 0
			switch VerdictState(e.State) {
		case VerdictBlacklisted:
			if e.SuccessStreak >= cfg.RehabSuccessThreshold {
				e.State = string(VerdictNormal)
				e.BlacklistedAtNs = 0
				becameNormal = true
			}
		case VerdictUnknown, "":
			e.State = string(VerdictNormal)
		}
	} else {
		e.LastResult = "fail"
		e.LastFailAtNs = now.UnixNano()
		e.FailureCount++
		e.SuccessStreak = 0
		if VerdictState(e.State) != VerdictBlacklisted && cfg.Enabled && cfg.Threshold > 0 && e.FailureCount >= cfg.Threshold {
			e.State = string(VerdictBlacklisted)
			e.BlacklistedAtNs = now.UnixNano()
			becameBlacklisted = true
		}
	}

	snapshot := *e
	onBlacklist := v.onBlacklist
	onRehab := v.onRehab
	v.mu.Unlock()

	if v.repo != nil {
		_ = v.repo.UpsertPlatformEgressIP(snapshot)
	}
	if becameBlacklisted && onBlacklist != nil {
		onBlacklist(platformID, ip)
	}
	if becameNormal && onRehab != nil {
		onRehab(platformID, ip)
	}
	return becameBlacklisted, becameNormal
}

// Snapshot returns a copy of the platform's ledger rows.
func (v *VerdictStore) Snapshot(platformID string) []model.PlatformEgressIP {
	if v == nil {
		return nil
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	m := v.byPlatform[platformID]
	out := make([]model.PlatformEgressIP, 0, len(m))
	for _, e := range m {
		out = append(out, *e)
	}
	return out
}

// GC removes ledger rows not seen since cutoff.
func (v *VerdictStore) GC(cutoff time.Time) (int64, error) {
	if v == nil {
		return 0, nil
	}
	cut := cutoff.UnixNano()
	v.mu.Lock()
	var removed int64
	for pid, m := range v.byPlatform {
		for ip, e := range m {
			if e.LastSeenNs > 0 && e.LastSeenNs < cut {
				delete(m, ip)
				removed++
			}
		}
		if len(m) == 0 {
			delete(v.byPlatform, pid)
		}
	}
	v.mu.Unlock()
	if v.repo != nil {
		if _, err := v.repo.GCPlatformEgressIPs(cut); err != nil {
			return removed, err
		}
	}
	return removed, nil
}

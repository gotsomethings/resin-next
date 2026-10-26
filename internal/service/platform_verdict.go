package service

import (
	"log"
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
	statusCode int,
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
	e.LastStatusCode = statusCode

	if pass {
		e.LastResult = "pass"
		e.LastSuccessAtNs = now.UnixNano()
		e.SuccessStreak++
		e.FailureCount = 0
			switch VerdictState(e.State) {
		case VerdictBlacklisted:
			// A manual blacklist is only cleared by hand, never by a streak.
			if !e.ManualBlacklist && e.SuccessStreak >= cfg.RehabSuccessThreshold {
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
		if err := v.repo.UpsertPlatformEgressIP(snapshot); err != nil {
			log.Printf("verdict: persist %s/%s failed: %v", platformID, ip, err)
		}
	}
	if becameBlacklisted && onBlacklist != nil {
		onBlacklist(platformID, ip)
	}
	if becameNormal && onRehab != nil {
		onRehab(platformID, ip)
	}
	return becameBlacklisted, becameNormal
}

// SetManualBlacklist forces ip into (on=true) or out of (on=false) the
// blacklist. A manual blacklist is never auto-rehabilitated; only a manual
// release clears it. It reports whether the stored state actually changed.
func (v *VerdictStore) SetManualBlacklist(platformID, ip string, on bool, now time.Time) bool {
	if v == nil {
		return false
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

	changed := false
	if on {
		if e.State != string(VerdictBlacklisted) || !e.ManualBlacklist {
			e.State = string(VerdictBlacklisted)
			e.ManualBlacklist = true
			e.BlacklistedAtNs = now.UnixNano()
			e.SuccessStreak = 0
			changed = true
		}
	} else if e.ManualBlacklist || e.State == string(VerdictBlacklisted) {
		e.ManualBlacklist = false
		e.BlacklistedAtNs = 0
		e.SuccessStreak = 0
		e.FailureCount = 0
		if e.LastProbeAtNs > 0 {
			e.State = string(VerdictNormal)
		} else {
			e.State = string(VerdictUnknown)
		}
		changed = true
	}

	snapshot := *e
	onBlacklist := v.onBlacklist
	onRehab := v.onRehab
	v.mu.Unlock()

	if !changed {
		return false
	}
	if v.repo != nil {
		_ = v.repo.UpsertPlatformEgressIP(snapshot)
	}
	if on {
		if onBlacklist != nil {
			onBlacklist(platformID, ip)
		}
	} else if onRehab != nil {
		onRehab(platformID, ip)
	}
	return true
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

// DeleteIP removes every ledger row for one egress IP across all platforms. It is
// used when the IP leaves the pool (e.g. its source was deleted) so that a later
// re-added source starts from a clean "untested" state instead of inheriting an
// old blacklist.
func (v *VerdictStore) DeleteIP(ip string) int {
	if v == nil || ip == "" {
		return 0
	}
	var pids []string
	v.mu.Lock()
	for pid, m := range v.byPlatform {
		if _, ok := m[ip]; ok {
			delete(m, ip)
			pids = append(pids, pid)
			if len(m) == 0 {
				delete(v.byPlatform, pid)
			}
		}
	}
	v.mu.Unlock()

	for _, pid := range pids {
		if v.repo != nil {
			if err := v.repo.DeletePlatformEgressIP(pid, ip); err != nil {
				log.Printf("verdict: delete %s/%s failed: %v", pid, ip, err)
			}
		}
	}
	return len(pids)
}

// GC removes ledger rows whose egress IP has not been seen for the retention
// window and is no longer present on any node. live reports current presence.
func (v *VerdictStore) GC(cutoff time.Time, live func(platformID, ip string) bool) (int64, error) {
	if v == nil {
		return 0, nil
	}
	cut := cutoff.UnixNano()
	type rowKey struct{ pid, ip string }
	var removedKeys []rowKey

	v.mu.Lock()
	for pid, m := range v.byPlatform {
		for ip, e := range m {
			if e.LastSeenNs <= 0 || e.LastSeenNs >= cut {
				continue
			}
			if live != nil && live(pid, ip) {
				continue
			}
			delete(m, ip)
			removedKeys = append(removedKeys, rowKey{pid: pid, ip: ip})
		}
		if len(m) == 0 {
			delete(v.byPlatform, pid)
		}
	}
	v.mu.Unlock()

	if v.repo != nil {
		for _, k := range removedKeys {
			if err := v.repo.DeletePlatformEgressIP(k.pid, k.ip); err != nil {
				return int64(len(removedKeys)), err
			}
		}
	}
	return int64(len(removedKeys)), nil
}

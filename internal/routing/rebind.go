package routing

import (
	"time"

	"github.com/Resinat/Resin/internal/node"
	"github.com/Resinat/Resin/internal/platform"
	"github.com/puzpuzpuz/xsync/v4"
)

// PlatformByID exposes a platform's runtime config to the proxy layer.
func (r *Router) PlatformByID(id string) (*platform.Platform, bool) {
	return r.pool.GetPlatform(id)
}

// RebindLease picks a live routable node other than failedHash and re-pins the
// account's sticky lease to it. Used by connect-stage failover (P4).
func (r *Router) RebindLease(platName, account string, failedHash node.Hash) (RouteResult, error) {
	plat, err := r.resolvePlatform(platName)
	if err != nil {
		return RouteResult{}, err
	}
	state := r.ensurePlatformState(plat.ID)

	var authorities []string
	if r.authorities != nil {
		authorities = r.authorities()
	}
	var p2cWindow time.Duration
	if r.p2cWindow != nil {
		p2cWindow = r.p2cWindow()
	}

	h, err := randomRouteExcluding(
		plat,
		state.IPLoadStats,
		r.pool,
		"",
		authorities,
		p2cWindow,
		failedHash,
	)
	if err != nil {
		return RouteResult{}, err
	}
	entry, ok := r.pool.GetEntry(h)
	if !ok {
		return RouteResult{}, ErrNoAvailableNodes
	}

	now := time.Now()
	nowNs := now.UnixNano()
	ttl := plat.StickyTTLNs
	if ttl <= 0 {
		ttl = int64(24 * time.Hour)
	}
	lease := Lease{
		NodeHash:       h,
		EgressIP:       entry.GetEgressIP(),
		CreatedAtNs:    nowNs,
		ExpiryNs:       now.Add(time.Duration(ttl)).UnixNano(),
		LastAccessedNs: nowNs,
	}

	if account != "" {
		_, _ = state.Leases.leases.Compute(account, func(current Lease, loaded bool) (Lease, xsync.ComputeOp) {
			if loaded {
				state.Leases.stats.Dec(current.EgressIP)
			}
			state.Leases.stats.Inc(lease.EgressIP)
			return lease, xsync.UpdateOp
		})
		r.emitLeaseEvent(LeaseEvent{
			Type:       LeaseReplace,
			PlatformID: plat.ID,
			Account:    account,
			NodeHash:   h,
			EgressIP:   lease.EgressIP,
		})
	}

	return withPlatformContext(plat, RouteResult{
		NodeHash:     h,
		EgressIP:     lease.EgressIP,
		LeaseCreated: true,
	}), nil
}

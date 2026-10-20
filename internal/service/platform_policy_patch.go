package service

import (
	"net/netip"
	"time"

	"github.com/Resinat/Resin/internal/platform"
)

// policyPatch is a partial update of platform.Policy. Nil pointers mean
// "field not present in the request".
type policyPatch struct {
	latencyTopN         *int
	latencyMaxMs        *int
	latencyP95MaxMs     *int
	minAliveStreak      *int
	minProbeSuccessRate *float64
	maxLastProbeAge     *time.Duration
	minNodeAge          *time.Duration
	egressIPVersion     *string

	egressCountryFilters []string
	hasCountryFilters    bool
	asnFilters           []string
	hasASNFilters        bool
	ipTypeFilters        []string
	hasIPTypeFilters     bool
	egressIPFilters      []string
	hasEgressIPFilters   bool
}

// policyPatchFromCreate converts CreatePlatformRequest policy fields.
func policyPatchFromCreate(req CreatePlatformRequest) policyPatch {
	p := policyPatch{
		latencyTopN:         req.LatencyTopN,
		latencyMaxMs:        req.LatencyMaxMs,
		latencyP95MaxMs:     req.LatencyP95MaxMs,
		minAliveStreak:      req.MinAliveStreak,
		minProbeSuccessRate: req.MinProbeSuccessRate,
		egressIPVersion:     req.EgressIPVersion,
	}
	if req.MaxLastProbeAge != nil {
		if d, err := time.ParseDuration(*req.MaxLastProbeAge); err == nil {
			p.maxLastProbeAge = &d
		}
	}
	if req.MinNodeAge != nil {
		if d, err := time.ParseDuration(*req.MinNodeAge); err == nil {
			p.minNodeAge = &d
		}
	}
	if req.EgressCountryFilters != nil {
		p.egressCountryFilters = req.EgressCountryFilters
		p.hasCountryFilters = true
	}
	if req.ASNFilters != nil {
		p.asnFilters = req.ASNFilters
		p.hasASNFilters = true
	}
	if req.IPTypeFilters != nil {
		p.ipTypeFilters = req.IPTypeFilters
		p.hasIPTypeFilters = true
	}
	if req.EgressIPFilters != nil {
		p.egressIPFilters = req.EgressIPFilters
		p.hasEgressIPFilters = true
	}
	return p
}

// policyPatchFromMerge extracts policy fields from a PATCH body.
func policyPatchFromMerge(patch mergePatch) (policyPatch, *ServiceError) {
	var p policyPatch

	if v, ok, err := patch.optionalInt("latency_top_n"); err != nil {
		return p, err
	} else if ok {
		p.latencyTopN = intPtr(v)
	}
	if v, ok, err := patch.optionalInt("latency_max_ms"); err != nil {
		return p, err
	} else if ok {
		p.latencyMaxMs = intPtr(v)
	}
	if v, ok, err := patch.optionalInt("latency_p95_max_ms"); err != nil {
		return p, err
	} else if ok {
		p.latencyP95MaxMs = intPtr(v)
	}
	if v, ok, err := patch.optionalInt("min_alive_streak"); err != nil {
		return p, err
	} else if ok {
		p.minAliveStreak = intPtr(v)
	}
	if v, ok, err := patch.optionalFloat("min_probe_success_rate"); err != nil {
		return p, err
	} else if ok {
		p.minProbeSuccessRate = floatPtr(v)
	}

	if d, ok, derr := patch.optionalDurationString("max_last_probe_age"); derr != nil {
		return p, derr
	} else if ok {
		p.maxLastProbeAge = &d
	}
	if d, ok, derr := patch.optionalDurationString("min_node_age"); derr != nil {
		return p, derr
	} else if ok {
		p.minNodeAge = &d
	}
	if v, ok, serr := patch.optionalString("egress_ip_version"); serr != nil {
		return p, serr
	} else if ok {
		p.egressIPVersion = &v
	}

	if v, ok, serr := patch.optionalStringSlice("egress_country_filters"); serr != nil {
		return p, serr
	} else if ok {
		p.egressCountryFilters = v
		p.hasCountryFilters = true
	}
	if v, ok, serr := patch.optionalStringSlice("asn_filters"); serr != nil {
		return p, serr
	} else if ok {
		p.asnFilters = v
		p.hasASNFilters = true
	}
	if v, ok, serr := patch.optionalStringSlice("ip_type_filters"); serr != nil {
		return p, serr
	} else if ok {
		p.ipTypeFilters = v
		p.hasIPTypeFilters = true
	}
	if v, ok, serr := patch.optionalStringSlice("egress_ip_filters"); serr != nil {
		return p, serr
	} else if ok {
		p.egressIPFilters = v
		p.hasEgressIPFilters = true
	}
	return p, nil
}

// applyPolicyPatch mutates dst with the provided partial policy.
func applyPolicyPatch(dst *platform.Policy, p policyPatch) *ServiceError {
	if p.latencyTopN != nil {
		dst.LatencyTopN = *p.latencyTopN
	}
	if p.latencyMaxMs != nil {
		dst.LatencyMaxMs = *p.latencyMaxMs
	}
	if p.latencyP95MaxMs != nil {
		dst.LatencyP95MaxMs = *p.latencyP95MaxMs
	}
	if p.minAliveStreak != nil {
		dst.MinAliveStreak = *p.minAliveStreak
	}
	if p.minProbeSuccessRate != nil {
		dst.MinProbeSuccessRate = *p.minProbeSuccessRate
	}
	if p.maxLastProbeAge != nil {
		dst.MaxLastProbeAge = *p.maxLastProbeAge
	}
	if p.minNodeAge != nil {
		dst.MinNodeAge = *p.minNodeAge
	}
	if p.egressIPVersion != nil {
		dst.EgressIPVersion = *p.egressIPVersion
	}
	if p.hasCountryFilters {
		dst.EgressCountryFilters = append([]string(nil), p.egressCountryFilters...)
	}
	if p.hasASNFilters {
		dst.ASNFilters = append([]string(nil), p.asnFilters...)
	}
	if p.hasIPTypeFilters {
		dst.IPTypeFilters = append([]string(nil), p.ipTypeFilters...)
	}
	if p.hasEgressIPFilters {
		dst.EgressIPFilters = append([]string(nil), p.egressIPFilters...)
	}
	return nil
}

// asnLookupFunc adapts the optional ASN service to the platform lookup type.
func (s *ControlPlaneService) asnLookupFunc() platform.ASNLookupFunc {
	if s == nil || s.ASN == nil {
		return nil
	}
	return func(addr netip.Addr) (platform.ASNInfo, bool) {
		info, ok := s.ASN.Lookup(addr)
		if !ok {
			return platform.ASNInfo{}, false
		}
		return platform.ASNInfo{ASN: info.Number, Org: info.Org}, true
	}
}

func intPtr(v int) *int           { return &v }
func floatPtr(v float64) *float64 { return &v }

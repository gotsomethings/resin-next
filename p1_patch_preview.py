import pathlib, sys

p = pathlib.Path(r'E:\Develop\resin\internal\service\control_plane_platform.go')
s = p.read_text(encoding='utf-8')

old_struct = '''type PlatformSpecFilter struct {
\tRegexFilters  []string `json:"regex_filters"`
\tRegionFilters []string `json:"region_filters"`
}'''
new_struct = '''type PlatformSpecFilter struct {
\tRegexFilters  []string `json:"regex_filters"`
\tRegionFilters []string `json:"region_filters"`

\tLatencyTopN          int      `json:"latency_top_n"`
\tLatencyMaxMs         int      `json:"latency_max_ms"`
\tLatencyP95MaxMs      int      `json:"latency_p95_max_ms"`
\tMinAliveStreak       int      `json:"min_alive_streak"`
\tMinProbeSuccessRate  float64  `json:"min_probe_success_rate"`
\tMaxLastProbeAge      string   `json:"max_last_probe_age"`
\tMinNodeAge           string   `json:"min_node_age"`
\tEgressIPVersion      string   `json:"egress_ip_version"`
\tEgressCountryFilters []string `json:"egress_country_filters"`
\tASNFilters           []string `json:"asn_filters"`
\tIPTypeFilters        []string `json:"ip_type_filters"`
\tEgressIPFilters      []string `json:"egress_ip_filters"`
}

// policy converts the preview spec into a platform policy.
func (f PlatformSpecFilter) policy() platform.Policy {
\tp := platform.Policy{
\t\tLatencyTopN:          f.LatencyTopN,
\t\tLatencyMaxMs:         f.LatencyMaxMs,
\t\tLatencyP95MaxMs:      f.LatencyP95MaxMs,
\t\tMinAliveStreak:       f.MinAliveStreak,
\t\tMinProbeSuccessRate:  f.MinProbeSuccessRate,
\t\tEgressIPVersion:      f.EgressIPVersion,
\t\tEgressCountryFilters: append([]string(nil), f.EgressCountryFilters...),
\t\tASNFilters:           append([]string(nil), f.ASNFilters...),
\t\tIPTypeFilters:        append([]string(nil), f.IPTypeFilters...),
\t\tEgressIPFilters:      append([]string(nil), f.EgressIPFilters...),
\t}
\tif f.MaxLastProbeAge != "" {
\t\tif d, err := time.ParseDuration(f.MaxLastProbeAge); err == nil {
\t\t\tp.MaxLastProbeAge = d
\t\t}
\t}
\tif f.MinNodeAge != "" {
\t\tif d, err := time.ParseDuration(f.MinNodeAge); err == nil {
\t\t\tp.MinNodeAge = d
\t\t}
\t}
\treturn p
}'''

if old_struct not in s:
    print('MISS struct'); sys.exit(1)
s = s.replace(old_struct, new_struct, 1)

marker = '// PreviewFilter returns nodes matching the given filter spec.'
idx = s.index(marker)

new_fn = '''// PreviewFilter returns nodes matching the given filter spec.
func (s *ControlPlaneService) PreviewFilter(req PreviewFilterRequest) ([]NodeSummary, error) {
	hasPlatformID := req.PlatformID != nil && *req.PlatformID != ""
	hasPlatformSpec := req.PlatformSpec != nil

	if hasPlatformID == hasPlatformSpec {
		return nil, invalidArg("exactly one of platform_id or platform_spec is required")
	}

	var regexFilters node.TagFilter
	var regionFilters []string
	var policy platform.Policy

	if hasPlatformID {
		plat, ok := s.Pool.GetPlatform(*req.PlatformID)
		if !ok {
			return nil, notFound("platform not found")
		}
		regexFilters = plat.RegexFilters
		regionFilters = append([]string(nil), plat.RegionFilters...)
		policy = plat.Policy
	} else {
		compiled, err := platform.CompileRegexFilters(req.PlatformSpec.RegexFilters)
		if err != nil {
			return nil, invalidArg(err.Error())
		}
		regexFilters = compiled
		regionFilters = req.PlatformSpec.RegionFilters
		if err := platform.ValidateRegionFilters(regionFilters); err != nil {
			return nil, invalidArg(err.Error())
		}
		policy = req.PlatformSpec.policy()
		if err := platform.Validate(policy); err != nil {
			return nil, invalidArg(err.Error())
		}
	}

	tmp := platform.NewConfiguredPlatformWithPolicy(
		"preview", "preview", regexFilters, regionFilters, policy,
		0, string(platform.ReverseProxyMissActionTreatAsEmpty),
		string(platform.ReverseProxyEmptyAccountBehaviorRandom), "", "BALANCED", false,
	)
	tmp.ASNLookup = s.asnLookupFunc()

	subLookup := s.Pool.MakeSubLookup()
	poolRange := func(fn func(node.Hash, *node.NodeEntry) bool) {
		s.Pool.Range(fn)
	}
	tmp.FullRebuild(poolRange, subLookup, s.geoLookup())

	var result []NodeSummary
	tmp.View().Range(func(h node.Hash) bool {
		entry, ok := s.Pool.GetEntry(h)
		if !ok {
			return true
		}
		result = append(result, s.nodeEntryToSummary(h, entry))
		return true
	})
	return result, nil
}

// geoLookup adapts the optional GeoIP service.
func (s *ControlPlaneService) geoLookup() platform.GeoLookupFunc {
	if s == nil || s.GeoIP == nil {
		return nil
	}
	return s.GeoIP.Lookup
}
'''

s = s[:idx] + new_fn
p.write_text(s, encoding='utf-8')
print('OK preview rewritten')

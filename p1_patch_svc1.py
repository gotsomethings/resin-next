import pathlib, sys

p = pathlib.Path(r'E:\Develop\resin\internal\service\control_plane_platform.go')
s = p.read_text(encoding='utf-8')

def rep(old, new, count=1):
    global s
    if s.count(old) != count:
        print('COUNT', s.count(old), repr(old[:80]))
        sys.exit(1)
    s = s.replace(old, new)

# 1) Any plain-string slice patch for egress_country_filters -> optionalStringSlice is fine ([]string)

# 2) platformToResponse: add policy fields
rep('''\tAllocationPolicy                 string   `json:"allocation_policy"`
\tPassiveCircuitBreakerDisabled    bool     `json:"passive_circuit_breaker_disabled"`
\tUpdatedAt                        string   `json:"updated_at"`
}''','''\tAllocationPolicy                 string   `json:"allocation_policy"`
\tPassiveCircuitBreakerDisabled    bool     `json:"passive_circuit_breaker_disabled"`
\tUpdatedAt                        string   `json:"updated_at"`

\t// Policy surface (P1).
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
}''')

rep('''\t\tAllocationPolicy:                 p.AllocationPolicy,
\t\tPassiveCircuitBreakerDisabled:    p.PassiveCircuitBreakerDisabled,
\t\tUpdatedAt:                        time.Unix(0, p.UpdatedAtNs).UTC().Format(time.RFC3339Nano),
\t}''','''\t\tAllocationPolicy:                 p.AllocationPolicy,
\t\tPassiveCircuitBreakerDisabled:    p.PassiveCircuitBreakerDisabled,
\t\tUpdatedAt:                        time.Unix(0, p.UpdatedAtNs).UTC().Format(time.RFC3339Nano),

\t\tLatencyTopN:          p.LatencyTopN,
\t\tLatencyMaxMs:         p.LatencyMaxMs,
\t\tLatencyP95MaxMs:      p.LatencyP95MaxMs,
\t\tMinAliveStreak:       p.MinAliveStreak,
\t\tMinProbeSuccessRate:  p.MinProbeSuccessRate,
\t\tMaxLastProbeAge:      time.Duration(p.MaxLastProbeAgeNs).String(),
\t\tMinNodeAge:           time.Duration(p.MinNodeAgeNs).String(),
\t\tEgressIPVersion:      p.EgressIPVersion,
\t\tEgressCountryFilters: append([]string(nil), p.EgressCountryFilters...),
\t\tASNFilters:           append([]string(nil), p.ASNFilters...),
\t\tIPTypeFilters:        append([]string(nil), p.IPTypeFilters...),
\t\tEgressIPFilters:      append([]string(nil), p.EgressIPFilters...),
\t}''')

# 3) platformConfig struct + conversions
rep('''type platformConfig struct {
\tName                             string
\tStickyTTLNs                      int64
\tRegexFilters                     []string
\tRegionFilters                    []string
\tReverseProxyMissAction           string
\tReverseProxyEmptyAccountBehavior string
\tReverseProxyFixedAccountHeader   string
\tAllocationPolicy                 string
\tPassiveCircuitBreakerDisabled    bool
}''','''type platformConfig struct {
\tName                             string
\tStickyTTLNs                      int64
\tRegexFilters                     []string
\tRegionFilters                    []string
\tReverseProxyMissAction           string
\tReverseProxyEmptyAccountBehavior string
\tReverseProxyFixedAccountHeader   string
\tAllocationPolicy                 string
\tPassiveCircuitBreakerDisabled    bool

\t// Policy surface (P1).
\tPolicy platform.Policy
}''')

rep('''\t\tAllocationPolicy:                 mp.AllocationPolicy,
\t\tPassiveCircuitBreakerDisabled:    mp.PassiveCircuitBreakerDisabled,
\t}
}''','''\t\tAllocationPolicy:                 mp.AllocationPolicy,
\t\tPassiveCircuitBreakerDisabled:    mp.PassiveCircuitBreakerDisabled,
\t\tPolicy:                           platform.PolicyFromModel(mp),
\t}
}''')

rep('''\treturn model.Platform{
\t\tID:                               id,
\t\tName:                             cfg.Name,
\t\tStickyTTLNs:                      cfg.StickyTTLNs,
\t\tRegexFilters:                     append([]string(nil), cfg.RegexFilters...),
\t\tRegionFilters:                    append([]string(nil), cfg.RegionFilters...),
\t\tReverseProxyMissAction:           cfg.ReverseProxyMissAction,
\t\tReverseProxyEmptyAccountBehavior: cfg.ReverseProxyEmptyAccountBehavior,
\t\tReverseProxyFixedAccountHeader:   cfg.ReverseProxyFixedAccountHeader,
\t\tAllocationPolicy:                 cfg.AllocationPolicy,
\t\tPassiveCircuitBreakerDisabled:    cfg.PassiveCircuitBreakerDisabled,
\t\tUpdatedAtNs:                      updatedAtNs,
\t}
}''','''\tmp := model.Platform{
\t\tID:                               id,
\t\tName:                             cfg.Name,
\t\tStickyTTLNs:                      cfg.StickyTTLNs,
\t\tRegexFilters:                     append([]string(nil), cfg.RegexFilters...),
\t\tRegionFilters:                    append([]string(nil), cfg.RegionFilters...),
\t\tReverseProxyMissAction:           cfg.ReverseProxyMissAction,
\t\tReverseProxyEmptyAccountBehavior: cfg.ReverseProxyEmptyAccountBehavior,
\t\tReverseProxyFixedAccountHeader:   cfg.ReverseProxyFixedAccountHeader,
\t\tAllocationPolicy:                 cfg.AllocationPolicy,
\t\tPassiveCircuitBreakerDisabled:    cfg.PassiveCircuitBreakerDisabled,
\t\tUpdatedAtNs:                      updatedAtNs,
\t}
\tplatform.ApplyPolicyToModel(cfg.Policy, &mp)
\treturn mp
}''')

rep('''\treturn platform.NewConfiguredPlatform(
\t\tid,
\t\tcfg.Name,
\t\tcompiledRegexFilters,
\t\tcfg.RegionFilters,
\t\tcfg.StickyTTLNs,
\t\tcfg.ReverseProxyMissAction,
\t\tcfg.ReverseProxyEmptyAccountBehavior,
\t\tcfg.ReverseProxyFixedAccountHeader,
\t\tcfg.AllocationPolicy,
\t\tcfg.PassiveCircuitBreakerDisabled,
\t), nil''','''\tplat := platform.NewConfiguredPlatformWithPolicy(
\t\tid,
\t\tcfg.Name,
\t\tcompiledRegexFilters,
\t\tcfg.RegionFilters,
\t\tcfg.Policy,
\t\tcfg.StickyTTLNs,
\t\tcfg.ReverseProxyMissAction,
\t\tcfg.ReverseProxyEmptyAccountBehavior,
\t\tcfg.ReverseProxyFixedAccountHeader,
\t\tcfg.AllocationPolicy,
\t\tcfg.PassiveCircuitBreakerDisabled,
\t)
\tif s != nil && s.ASN != nil {
\t\tplat.ASNLookup = s.asnLookupFunc()
\t}
\treturn plat, nil''')

# 4) validatePlatformConfig: add policy validation
rep('''\tif err := validatePlatformEmptyAccountConfig(cfg); err != nil {
\t\treturn err
\t}
\treturn nil
}''','''\tif err := validatePlatformEmptyAccountConfig(cfg); err != nil {
\t\treturn err
\t}
\tif err := platform.Validate(cfg.Policy); err != nil {
\t\treturn invalidArg(err.Error())
\t}
\treturn nil
}''')

p.write_text(s, encoding='utf-8')
print('OK platform service part 1')

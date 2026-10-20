import pathlib, sys

p = pathlib.Path(r'E:\Develop\resin\internal\service\control_plane_platform.go')
s = p.read_text(encoding='utf-8')

def rep(old, new, count=1):
    global s
    if s.count(old) != count:
        print('COUNT', s.count(old), repr(old[:80]))
        sys.exit(1)
    s = s.replace(old, new)

# CreatePlatformRequest policy fields
rep('''\tAllocationPolicy                 *string  `json:"allocation_policy"`
\tPassiveCircuitBreakerDisabled    *bool    `json:"passive_circuit_breaker_disabled"`
}''','''\tAllocationPolicy                 *string  `json:"allocation_policy"`
\tPassiveCircuitBreakerDisabled    *bool    `json:"passive_circuit_breaker_disabled"`

\tLatencyTopN          *int     `json:"latency_top_n"`
\tLatencyMaxMs         *int     `json:"latency_max_ms"`
\tLatencyP95MaxMs      *int     `json:"latency_p95_max_ms"`
\tMinAliveStreak       *int     `json:"min_alive_streak"`
\tMinProbeSuccessRate  *float64 `json:"min_probe_success_rate"`
\tMaxLastProbeAge      *string  `json:"max_last_probe_age"`
\tMinNodeAge           *string  `json:"min_node_age"`
\tEgressIPVersion      *string  `json:"egress_ip_version"`
\tEgressCountryFilters []string `json:"egress_country_filters"`
\tASNFilters           []string `json:"asn_filters"`
\tIPTypeFilters        []string `json:"ip_type_filters"`
\tEgressIPFilters      []string `json:"egress_ip_filters"`
}''')

# CreatePlatform overlay
rep('''\tif req.PassiveCircuitBreakerDisabled != nil {
\t\tcfg.PassiveCircuitBreakerDisabled = *req.PassiveCircuitBreakerDisabled
\t}
\tif err := validatePlatformConfig(&cfg, true); err != nil {''','''\tif req.PassiveCircuitBreakerDisabled != nil {
\t\tcfg.PassiveCircuitBreakerDisabled = *req.PassiveCircuitBreakerDisabled
\t}
\tif svcErr := applyPolicyPatch(&cfg.Policy, policyPatchFromCreate(req)); svcErr != nil {
\t\treturn nil, svcErr
\t}
\tif err := validatePlatformConfig(&cfg, true); err != nil {''')

# UpdatePlatform: apply policy patch
rep('''\tif disabled, ok, err := patch.optionalBool("passive_circuit_breaker_disabled"); err != nil {
\t\treturn nil, err
\t} else if ok {
\t\tcfg.PassiveCircuitBreakerDisabled = disabled
\t}
\tif err := validatePlatformConfig(&cfg, regionFiltersPatched); err != nil {''','''\tif disabled, ok, err := patch.optionalBool("passive_circuit_breaker_disabled"); err != nil {
\t\treturn nil, err
\t} else if ok {
\t\tcfg.PassiveCircuitBreakerDisabled = disabled
\t}
\tif svcErr := applyPolicyPatch(&cfg.Policy, policyPatchFromMerge(patch)); svcErr != nil {
\t\treturn nil, svcErr
\t}
\tif err := validatePlatformConfig(&cfg, regionFiltersPatched); err != nil {''')

p.write_text(s, encoding='utf-8')
print('OK svc part 2')

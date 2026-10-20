import pathlib, sys

p = pathlib.Path(r'E:\Develop\resin\internal\state\repo_state.go')
s = p.read_text(encoding='utf-8')

# 1) platformColumns: extend
old = '''\tegress_ip_version, egress_country_filters_json, asn_filters_json,
\tip_type_filters_json, egress_ip_filters_json`'''
new = '''\tegress_ip_version, egress_country_filters_json, asn_filters_json,
\tip_type_filters_json, egress_ip_filters_json,
\tprobe_enabled, probe_url, probe_method, probe_headers, probe_body,
\tprobe_pass_status_json, probe_follow_redirects, probe_interval_ns, probe_scope,
\tverdict_enabled, blacklist_threshold, rehab_interval_ns, rehab_success_threshold,
\tblacklist_empty_policy, blacklist_grace_k`'''
if old not in s:
    sys.exit('MISS cols')
s = s.replace(old, new, 1)

# 2) scanPlatform: extend
old = '''\tvar countryFiltersJSON, asnFiltersJSON, ipTypeFiltersJSON, ipFiltersJSON string
\tvar passiveCircuitBreakerDisabled int'''
new = '''\tvar countryFiltersJSON, asnFiltersJSON, ipTypeFiltersJSON, ipFiltersJSON string
\tvar passStatusJSON string
\tvar probeEnabled, probeFollowRedirects, verdictEnabled int
\tvar passiveCircuitBreakerDisabled int'''
if old not in s:
    sys.exit('MISS scan vars')
s = s.replace(old, new, 1)

old = '''\t\t&p.EgressIPVersion, &countryFiltersJSON, &asnFiltersJSON,
\t\t&ipTypeFiltersJSON, &ipFiltersJSON,
\t); err != nil {'''
new = '''\t\t&p.EgressIPVersion, &countryFiltersJSON, &asnFiltersJSON,
\t\t&ipTypeFiltersJSON, &ipFiltersJSON,
\t\t&probeEnabled, &p.ProbeURL, &p.ProbeMethod, &p.ProbeHeaders, &p.ProbeBody,
\t\t&passStatusJSON, &probeFollowRedirects, &p.ProbeIntervalNs, &p.ProbeScope,
\t\t&verdictEnabled, &p.BlacklistThreshold, &p.RehabIntervalNs, &p.RehabSuccessThreshold,
\t\t&p.BlacklistEmptyPolicy, &p.BlacklistGraceK,
\t); err != nil {'''
if old not in s:
    sys.exit('MISS scan args')
s = s.replace(old, new, 1)

old = '''\tp.PassiveCircuitBreakerDisabled = passiveCircuitBreakerDisabled != 0
\tvar err error'''
new = '''\tp.PassiveCircuitBreakerDisabled = passiveCircuitBreakerDisabled != 0
\tp.ProbeEnabled = probeEnabled != 0
\tp.ProbeFollowRedirects = probeFollowRedirects != 0
\tp.VerdictEnabled = verdictEnabled != 0
\tif p.ProbePassStatus, err = decodeIntSliceJSON(passStatusJSON); err != nil {
\t\treturn nil, fmt.Errorf("decode platform %s probe_pass_status_json: %w", p.ID, err)
\t}
\tvar err error'''
if old not in s:
    sys.exit('MISS scan post')
s = s.replace(old, new, 1)

p.write_text(s, encoding='utf-8')
print('OK scanPlatform')

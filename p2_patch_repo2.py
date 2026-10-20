import pathlib, sys

p = pathlib.Path(r'E:\Develop\resin\internal\state\repo_state.go')
s = p.read_text(encoding='utf-8')

# upsert: add columns + placeholders
old = '''\t\t                       egress_ip_version, egress_country_filters_json, asn_filters_json,
\t\t                       ip_type_filters_json, egress_ip_filters_json)
\t\tVALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)'''
new = '''\t\t                       egress_ip_version, egress_country_filters_json, asn_filters_json,
\t\t                       ip_type_filters_json, egress_ip_filters_json,
\t\t                       probe_enabled, probe_url, probe_method, probe_headers, probe_body,
\t\t                       probe_pass_status_json, probe_follow_redirects, probe_interval_ns, probe_scope,
\t\t                       verdict_enabled, blacklist_threshold, rehab_interval_ns, rehab_success_threshold,
\t\t                       blacklist_empty_policy, blacklist_grace_k)
\t\tVALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)'''
if old not in s:
    sys.exit('MISS upsert head')
s = s.replace(old, new, 1)

old = '''\t\t\tegress_ip_filters_json   = excluded.egress_ip_filters_json'''
new = '''\t\t\tegress_ip_filters_json   = excluded.egress_ip_filters_json,
\t\t\tprobe_enabled            = excluded.probe_enabled,
\t\t\tprobe_url                = excluded.probe_url,
\t\t\tprobe_method             = excluded.probe_method,
\t\t\tprobe_headers            = excluded.probe_headers,
\t\t\tprobe_body               = excluded.probe_body,
\t\t\tprobe_pass_status_json   = excluded.probe_pass_status_json,
\t\t\tprobe_follow_redirects   = excluded.probe_follow_redirects,
\t\t\tprobe_interval_ns        = excluded.probe_interval_ns,
\t\t\tprobe_scope              = excluded.probe_scope,
\t\t\tverdict_enabled          = excluded.verdict_enabled,
\t\t\tblacklist_threshold      = excluded.blacklist_threshold,
\t\t\trehab_interval_ns        = excluded.rehab_interval_ns,
\t\t\trehab_success_threshold  = excluded.rehab_success_threshold,
\t\t\tblacklist_empty_policy   = excluded.blacklist_empty_policy,
\t\t\tblacklist_grace_k        = excluded.blacklist_grace_k'''
if old not in s:
    sys.exit('MISS upsert set')
s = s.replace(old, new, 1)

old = '''\t\tp.EgressIPVersion, countryFiltersJSON, asnFiltersJSON,
\t\tipTypeFiltersJSON, ipFiltersJSON)'''
new = '''\t\tp.EgressIPVersion, countryFiltersJSON, asnFiltersJSON,
\t\tipTypeFiltersJSON, ipFiltersJSON,
\t\tboolToInt(p.ProbeEnabled), p.ProbeURL, p.ProbeMethod, p.ProbeHeaders, p.ProbeBody,
\t\tpassStatusJSON, boolToInt(p.ProbeFollowRedirects), p.ProbeIntervalNs, p.ProbeScope,
\t\tboolToInt(p.VerdictEnabled), p.BlacklistThreshold, p.RehabIntervalNs, p.RehabSuccessThreshold,
\t\tp.BlacklistEmptyPolicy, p.BlacklistGraceK)'''
if old not in s:
    sys.exit('MISS upsert args')
s = s.replace(old, new, 1)

# encode pass status before lock
old = '''\tipFiltersJSON, err := encodeStringSliceJSON(p.EgressIPFilters)
\tif err != nil {
\t\treturn fmt.Errorf("encode platform %s egress_ip_filters: %w", p.ID, err)
\t}'''
new = '''\tipFiltersJSON, err := encodeStringSliceJSON(p.EgressIPFilters)
\tif err != nil {
\t\treturn fmt.Errorf("encode platform %s egress_ip_filters: %w", p.ID, err)
\t}
\tpassStatusJSON, err := encodeIntSliceJSON(p.ProbePassStatus)
\tif err != nil {
\t\treturn fmt.Errorf("encode platform %s probe_pass_status: %w", p.ID, err)
\t}'''
if old not in s:
    sys.exit('MISS encode pass')
s = s.replace(old, new, 1)

# helpers: decodeIntSliceJSON/encodeIntSliceJSON/boolToInt near string slice helpers
old = '''// --- system_config ---'''
new = '''func encodeIntSliceJSON(values []int) (string, error) {
\tif values == nil {
\t\tvalues = []int{}
\t}
\tdata, err := json.Marshal(values)
\tif err != nil {
\t\treturn "", err
\t}
\treturn string(data), nil
}

func decodeIntSliceJSON(raw string) ([]int, error) {
\tvar out []int
\tif err := json.Unmarshal([]byte(raw), &out); err != nil {
\t\treturn nil, err
\t}
\tif out == nil {
\t\tout = []int{}
\t}
\treturn out, nil
}

func boolToInt(v bool) int {
\tif v {
\t\treturn 1
\t}
\treturn 0
}

// --- system_config ---'''
if old not in s:
    sys.exit('MISS helpers anchor')
s = s.replace(old, new, 1)

p.write_text(s, encoding='utf-8')
print('OK upsert')

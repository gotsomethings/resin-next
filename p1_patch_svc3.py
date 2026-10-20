import pathlib

# 1) append optionalFloat to patch_helpers.go
p = pathlib.Path(r'E:\Develop\resin\internal\service\patch_helpers.go')
s = p.read_text(encoding='utf-8')
if 'func (p mergePatch) optionalFloat' not in s:
    s += '''

func (p mergePatch) optionalFloat(field string) (float64, bool, *ServiceError) {
	raw, ok := p[field]
	if !ok {
		return 0, false, nil
	}
	value, ok := raw.(float64)
	if !ok {
		return 0, true, invalidArg(fmt.Sprintf("%s: must be a number", field))
	}
	return value, true, nil
}
'''
    p.write_text(s, encoding='utf-8')
    print('OK optionalFloat')
else:
    print('optionalFloat exists')

# 2) ControlPlaneService: add ASN field
p2 = pathlib.Path(r'E:\Develop\resin\internal\service\control_plane_system.go')
s2 = p2.read_text(encoding='utf-8')
if 'ASN             *geoip.ASNService' not in s2:
    old = '\tGeoIP           *geoip.Service\n'
    new = '\tGeoIP           *geoip.Service\n\tASN             *geoip.ASNService\n'
    if old not in s2:
        raise SystemExit('MISS GeoIP field')
    s2 = s2.replace(old, new, 1)
    p2.write_text(s2, encoding='utf-8')
    print('OK ASN field')
else:
    print('ASN field exists')

# 3) platform patch whitelist: add 12 policy fields
old = '''var platformPatchAllowedFields = map[string]bool{
\t"name":                                 true,
\t"sticky_ttl":                           true,
\t"regex_filters":                        true,
\t"region_filters":                       true,
\t"reverse_proxy_miss_action":            true,
\t"reverse_proxy_empty_account_behavior": true,
\t"reverse_proxy_fixed_account_header":   true,
\t"allocation_policy":                    true,
\t"passive_circuit_breaker_disabled":     true,
}'''
new = '''var platformPatchAllowedFields = map[string]bool{
\t"name":                                 true,
\t"sticky_ttl":                           true,
\t"regex_filters":                        true,
\t"region_filters":                       true,
\t"reverse_proxy_miss_action":            true,
\t"reverse_proxy_empty_account_behavior": true,
\t"reverse_proxy_fixed_account_header":   true,
\t"allocation_policy":                    true,
\t"passive_circuit_breaker_disabled":     true,

\t// Policy surface (P1).
\t"latency_top_n":           true,
\t"latency_max_ms":          true,
\t"latency_p95_max_ms":      true,
\t"min_alive_streak":        true,
\t"min_probe_success_rate":  true,
\t"max_last_probe_age":      true,
\t"min_node_age":            true,
\t"egress_ip_version":       true,
\t"egress_country_filters":  true,
\t"asn_filters":             true,
\t"ip_type_filters":         true,
\t"egress_ip_filters":       true,
}'''
if old not in s2:
    raise SystemExit('MISS platformPatchAllowedFields')
s2 = s2.replace(old, new, 1)
p2.write_text(s2, encoding='utf-8')
print('OK platform patch whitelist')

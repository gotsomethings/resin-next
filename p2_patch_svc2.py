import pathlib, sys

# 1) optionalIntSlice
p = pathlib.Path(r'E:\Develop\resin\internal\service\patch_helpers.go')
s = p.read_text(encoding='utf-8')
if 'func (p mergePatch) optionalIntSlice' not in s:
    s += '''

func (p mergePatch) optionalIntSlice(field string) ([]int, bool, *ServiceError) {
\traw, ok := p[field]
\tif !ok {
\t\treturn nil, false, nil
\t}
\tarr, ok := raw.([]any)
\tif !ok {
\t\treturn nil, true, invalidArg(fmt.Sprintf("%s: must be an array", field))
\t}
\tvalue := make([]int, len(arr))
\tfor i, item := range arr {
\t\tnum, ok := item.(float64)
\t\tif !ok || math.Trunc(num) != num {
\t\t\treturn nil, true, invalidArg(fmt.Sprintf("%s[%d]: must be an integer", field, i))
\t\t}
\t\tvalue[i] = int(num)
\t}
\treturn value, true, nil
}
'''
    p.write_text(s, encoding='utf-8')
    print('OK optionalIntSlice')

# 2) whitelist
p2 = pathlib.Path(r'E:\Develop\resin\internal\service\control_plane_system.go')
s2 = p2.read_text(encoding='utf-8')
old = '''\t"egress_ip_filters":       true,
}'''
new = '''\t"egress_ip_filters":       true,

\t// Probe / verdict surface (P2).
\t"probe_enabled":           true,
\t"probe_url":               true,
\t"probe_method":            true,
\t"probe_headers":           true,
\t"probe_body":              true,
\t"probe_pass_status":       true,
\t"probe_follow_redirects":  true,
\t"probe_interval":          true,
\t"probe_scope":             true,
\t"verdict_enabled":         true,
\t"blacklist_threshold":     true,
\t"rehab_interval":          true,
\t"rehab_success_threshold": true,
\t"blacklist_empty_policy":  true,
\t"blacklist_grace_k":       true,
}'''
if old not in s2:
    sys.exit('MISS whitelist')
s2 = s2.replace(old, new, 1)
p2.write_text(s2, encoding='utf-8')
print('OK whitelist')

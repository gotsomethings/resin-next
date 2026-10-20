import pathlib, re

p = pathlib.Path(r'E:\Develop\resin\internal\service\platform_policy_patch.go')
s = p.read_text(encoding='utf-8')

if 'func intPtr' not in s:
    s += '''
func intPtr(v int) *int           { return &v }
func floatPtr(v float64) *float64 { return &v }
'''

for field, fn, typ in [
    ('p.latencyTopN', 'optionalInt', 'int'),
    ('p.latencyMaxMs', 'optionalInt', 'int'),
    ('p.latencyP95MaxMs', 'optionalInt', 'int'),
    ('p.minAliveStreak', 'optionalInt', 'int'),
    ('p.minProbeSuccessRate', 'optionalFloat', 'float64'),
]:
    name = field.replace('p.', '')
    old = f'\tif {field}, _, err = patch.{fn}("{{"}} ", err != nil {{\n\t\treturn p, err\n\t}}'

# simpler: regex replace the assignment pattern
pat = re.compile(r'\tif (p\.\w+), _, err = patch\.optional(Int|Float)\("([a-z0-9_]+)"\); err != nil \{\n\t\treturn p, err\n\t\}')

def sub(m):
    field, kind, name = m.group(1), m.group(2), m.group(3)
    conv = 'intPtr' if kind == 'Int' else 'floatPtr'
    fname = 'optionalInt' if kind == 'Int' else 'optionalFloat'
    return (f'\tif v, ok, err := patch.{fname}("{name}"); err != nil {{\n'
            f'\t\treturn p, err\n'
            f'\t}} else if ok {{\n'
            f'\t\t{field} = {conv}(v)\n'
            f'\t}}')

s2, n = pat.subn(sub, s)
print('replaced', n)
p.write_text(s2, encoding='utf-8')

# control_plane_platform.go: remove unused netip import, fix applyPolicyPatch call sites
p2 = pathlib.Path(r'E:\Develop\resin\internal\service\control_plane_platform.go')
s2 = p2.read_text(encoding='utf-8')
s2 = s2.replace('import (\n\t"encoding/json"\n\t"net/netip"', 'import (\n\t"encoding/json"', 1)

s2 = s2.replace('''\tif svcErr := applyPolicyPatch(&cfg.Policy, policyPatchFromMerge(patch)); svcErr != nil {
\t\treturn nil, svcErr
\t}''', '''\tpolicyPatch, svcErr := policyPatchFromMerge(patch)
\tif svcErr != nil {
\t\treturn nil, svcErr
\t}
\tif svcErr := applyPolicyPatch(&cfg.Policy, policyPatch); svcErr != nil {
\t\treturn nil, svcErr
\t}''')

s2 = s2.replace('''\tif s != nil && s.ASN != nil {
\t\tplat.ASNLookup = s.asnLookupFunc()
\t}''', '''\tif s := p.owner(); s != nil {
\t\tplat.ASNLookup = s.asnLookupFunc()
\t}''')

p2.write_text(s2, encoding='utf-8')
print('OK platform file fixed')

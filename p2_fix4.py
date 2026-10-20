import pathlib, re

# 1) netip import
p = pathlib.Path(r'E:\Develop\resin\internal\service\control_plane_platform.go')
s = p.read_text(encoding='utf-8')
if '"net/netip"' not in s:
    s = s.replace('import (\n\t"encoding/json"', 'import (\n\t"encoding/json"\n\t"net/netip"', 1)
    p.write_text(s, encoding='utf-8')
    print('OK netip')

# 2) pointer helpers + six assignments
p2 = pathlib.Path(r'E:\Develop\resin\internal\service\platform_probe_patch.go')
s2 = p2.read_text(encoding='utf-8')

if 'func boolPtr' not in s2:
    s2 += '''
func boolPtr(v bool) *bool { return &v }
'''

repls = [
 ('\tif p.enabled, _, err = patch.optionalBool("probe_enabled"); err != nil {\n\t\treturn p, err\n\t}',
  '\tif v, ok, serr := patch.optionalBool("probe_enabled"); serr != nil {\n\t\treturn p, serr\n\t} else if ok {\n\t\tp.enabled = boolPtr(v)\n\t}'),
 ('\tif p.followRedirects, _, err = patch.optionalBool("probe_follow_redirects"); err != nil {\n\t\treturn p, err\n\t}',
  '\tif v, ok, serr := patch.optionalBool("probe_follow_redirects"); serr != nil {\n\t\treturn p, serr\n\t} else if ok {\n\t\tp.followRedirects = boolPtr(v)\n\t}'),
 ('\tif p.verdictEnabled, _, err = patch.optionalBool("verdict_enabled"); err != nil {\n\t\treturn p, err\n\t}',
  '\tif v, ok, serr := patch.optionalBool("verdict_enabled"); serr != nil {\n\t\treturn p, serr\n\t} else if ok {\n\t\tp.verdictEnabled = boolPtr(v)\n\t}'),
 ('\tif p.blacklistThreshold, _, err = patch.optionalInt("blacklist_threshold"); err != nil {\n\t\treturn p, err\n\t}',
  '\tif v, ok, serr := patch.optionalInt("blacklist_threshold"); serr != nil {\n\t\treturn p, serr\n\t} else if ok {\n\t\tp.blacklistThreshold = intPtr(v)\n\t}'),
 ('\tif p.rehabSuccessThreshold, _, err = patch.optionalInt("rehab_success_threshold"); err != nil {\n\t\treturn p, err\n\t}',
  '\tif v, ok, serr := patch.optionalInt("rehab_success_threshold"); serr != nil {\n\t\treturn p, serr\n\t} else if ok {\n\t\tp.rehabSuccessThreshold = intPtr(v)\n\t}'),
 ('\tif p.blacklistGraceK, _, err = patch.optionalInt("blacklist_grace_k"); err != nil {\n\t\treturn p, err\n\t}',
  '\tif v, ok, serr := patch.optionalInt("blacklist_grace_k"); serr != nil {\n\t\treturn p, serr\n\t} else if ok {\n\t\tp.blacklistGraceK = intPtr(v)\n\t}'),
]
for old, new in repls:
    if old not in s2:
        print('MISS:', old[:60]); continue
    s2 = s2.replace(old, new, 1)

# remove now-unused err var if present
s2 = s2.replace('\tvar p probePatch\n\tvar err *ServiceError\n', '\tvar p probePatch\n', 1)

p2.write_text(s2, encoding='utf-8')
print('OK probe patch pointers')

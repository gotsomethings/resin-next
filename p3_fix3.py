import pathlib, sys

p = pathlib.Path(r'E:\Develop\resin\internal\service\control_plane_system.go')
s = p.read_text(encoding='utf-8')
old = '''\t"probe_concurrency":                        true,
\t"probe_concurrency_mode":                   true,
}'''
new = '''\t"probe_concurrency":                        true,
\t"probe_concurrency_mode":                   true,
\t// Read-only echo of the governor's live value; accepted but overwritten.
\t"probe_concurrency_effective":              true,
}'''
if old not in s:
    sys.exit('MISS allowlist')
s = s.replace(old, new, 1)

old2 = '''\t// 4. Additional validation.
\tif err := validateRuntimeConfig(newCfg); err != nil {
\t\treturn nil, err
\t}'''
new2 = '''\t// probe_concurrency_effective is a read-only echo: never trust the body.
\tif s.ProbeMgr != nil {
\t\tnewCfg.ProbeConcurrencyEffective = s.ProbeMgr.EffectiveConcurrency()
\t}

\t// 4. Additional validation.
\tif err := validateRuntimeConfig(newCfg); err != nil {
\t\treturn nil, err
\t}'''
if old2 not in s:
    sys.exit('MISS force anchor')
s = s.replace(old2, new2, 1)
p.write_text(s, encoding='utf-8')
print('OK allowlist')

import pathlib, sys

# 1) RuntimeConfig fields
p = pathlib.Path(r'E:\Develop\resin\internal\config\runtime.go')
s = p.read_text(encoding='utf-8')
if 'ProbeConcurrencyMode' not in s:
    old = '''\t// Persistence
\tCacheFlushInterval       Duration `json:"cache_flush_interval"`
\tCacheFlushDirtyThreshold int      `json:"cache_flush_dirty_threshold"`
}'''
    new = '''\t// Persistence
\tCacheFlushInterval       Duration `json:"cache_flush_interval"`
\tCacheFlushDirtyThreshold int      `json:"cache_flush_dirty_threshold"`

\t// Probe concurrency governance (P3).
\t// SettingsConcurrency is the configured ceiling; only the governor moves
\t// EffectiveConcurrency, and it never drops below SettingsConcurrency/10.
\tProbeConcurrencySettings int    `json:"probe_concurrency"`
\tProbeConcurrencyMode     string `json:"probe_concurrency_mode"`
\tProbeConcurrencyEffective int   `json:"probe_concurrency_effective"`
}'''
    if old not in s:
        sys.exit('MISS runtime fields')
    s = s.replace(old, new, 1)

    old2 = '''\t\tCacheFlushInterval:       Duration(5 * time.Minute),
\t\tCacheFlushDirtyThreshold: 1000,
\t}'''
    new2 = '''\t\tCacheFlushInterval:       Duration(5 * time.Minute),
\t\tCacheFlushDirtyThreshold: 1000,

\t\tProbeConcurrencySettings:  1000,
\t\tProbeConcurrencyMode:      "fixed",
\t\tProbeConcurrencyEffective: 1000,
\t}'''
    if old2 not in s:
        sys.exit('MISS runtime defaults')
    s = s.replace(old2, new2, 1)
    p.write_text(s, encoding='utf-8')
    print('OK runtime.go')

# 2) patch whitelist for runtime config
p2 = pathlib.Path(r'E:\Develop\resin\internal\service\control_plane_system.go')
s2 = p2.read_text(encoding='utf-8')
old = '''\t"cache_flush_dirty_threshold":              true,
}'''
new = '''\t"cache_flush_dirty_threshold":              true,
\t"probe_concurrency":                        true,
\t"probe_concurrency_mode":                   true,
}'''
if old not in s2:
    sys.exit('MISS runtime whitelist')
s2 = s2.replace(old, new, 1)

# 3) validate runtime config additions
old2 = '''\tif cfg.CacheFlushDirtyThreshold < 0 {
\t\treturn invalidArg("cache_flush_dirty_threshold: must be non-negative")
\t}'''
new2 = '''\tif cfg.CacheFlushDirtyThreshold < 0 {
\t\treturn invalidArg("cache_flush_dirty_threshold: must be non-negative")
\t}
\tif cfg.ProbeConcurrencySettings < 1 || cfg.ProbeConcurrencySettings > 10000 {
\t\treturn invalidArg("probe_concurrency: must be between 1 and 10000")
\t}
\tswitch cfg.ProbeConcurrencyMode {
\tcase "fixed", "auto":
\tdefault:
\t\treturn invalidArg("probe_concurrency_mode: must be fixed or auto")
\t}'''
if old2 not in s2:
    sys.exit('MISS runtime validate')
s2 = s2.replace(old2, new2, 1)

# 4) after atomic swap, push into probe manager
old3 = '''\t// 6. Atomic swap.
\ts.RuntimeCfg.Store(newCfg)
\ts.configVersion = newVersion

\treturn newCfg, nil'''
new3 = '''\t// 6. Atomic swap.
\ts.RuntimeCfg.Store(newCfg)
\ts.configVersion = newVersion

\t// 7. Hot-apply concurrency settings (fixed mode takes effect immediately;
\t//    auto mode lets the governor converge from the new ceiling).
\tif s.ProbeMgr != nil {
\t\ts.ProbeMgr.SetSettingsConcurrency(newCfg.ProbeConcurrencySettings)
\t\tif newCfg.ProbeConcurrencyMode == "fixed" {
\t\t\ts.ProbeMgr.SetEffectiveConcurrency(newCfg.ProbeConcurrencySettings)
\t\t}
\t}

\treturn newCfg, nil'''
if old3 not in s2:
    sys.exit('MISS swap anchor')
s2 = s2.replace(old3, new3, 1)
p2.write_text(s2, encoding='utf-8')
print('OK system config')

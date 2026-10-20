import pathlib

# Fix default platform config: seed probe/verdict defaults
p = pathlib.Path(r'E:\Develop\resin\internal\service\control_plane_platform.go')
s = p.read_text(encoding='utf-8')
old = '''\t\tAllocationPolicy: s.EnvCfg.DefaultPlatformAllocationPolicy,
\t}
}'''
new = '''\t\tAllocationPolicy: s.EnvCfg.DefaultPlatformAllocationPolicy,
\t\tProbe:            platform.ProbeFromModel(model.Platform{}),
\t\tVerdict:          platform.VerdictFromModel(model.Platform{}),
\t}
}'''
if old not in s:
    raise SystemExit('MISS defaultPlatformConfig')
s = s.replace(old, new, 1)
p.write_text(s, encoding='utf-8')
print('OK default probe')

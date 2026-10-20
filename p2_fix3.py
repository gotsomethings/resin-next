import pathlib
p = pathlib.Path(r'E:\Develop\resin\internal\probe\platform_probe.go')
s = p.read_text(encoding='utf-8')
s = s.replace('\t\tcase string(platform.VerdictUnknownForProbe()):\n\t\t\tdue = true\n', '', 1)
p.write_text(s, encoding='utf-8')
print('OK removed stale case')

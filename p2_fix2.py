import pathlib
p = pathlib.Path(r'E:\Develop\resin\internal\probe\platform_probe.go')
s = p.read_text(encoding='utf-8')
s = s.replace('\t\tcase string(platform.VerdictUnknownForProbe()):\n\t\t\tdue = true\n', '')
s = s.replace('\t"fmt"\n', '')
s = s.replace('\nvar _ = fmt.Sprintf\n', '')
p.write_text(s, encoding='utf-8')
print('OK probe fix')

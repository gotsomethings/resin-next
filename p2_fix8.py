import pathlib, re
p = pathlib.Path(r'E:\Develop\resin\internal\service\platform_verdict.go')
s = p.read_text(encoding='utf-8')
s = re.sub(r'\tfmt\.Printf\("DEBUG verdict[^\n]*\n', '', s)
s = s.replace('import (\n\t"fmt"\n\t"net/netip"', 'import (\n\t"net/netip"', 1)
p.write_text(s, encoding='utf-8')
print('OK debug removed')

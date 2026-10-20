import pathlib
p = pathlib.Path(r'E:\Develop\resin\internal\platform\filters.go')
s = p.read_text(encoding='utf-8')
old = '''\tfor _, r := range rules {
\t\thit := false
\t\tif r.isCIDR {
\t\t\thit = r.prefix.Contains(ip)
\t\t} else {
\t\t\thit = r.addr == ip
\t\t}
\t\tif !hit {
\t\t\tcontinue
\t\t}
\t\tif r.deny {
\t\t\treturn false
\t\t}
\t\thasInclude = true
\t\tincluded = true
\t}
\treturn !hasInclude || included'''
new = '''\tfor _, r := range rules {
\t\thit := false
\t\tif r.isCIDR {
\t\t\thit = r.prefix.Contains(ip)
\t\t} else {
\t\t\thit = r.addr == ip
\t\t}
\t\tif r.deny {
\t\t\tif hit {
\t\t\t\treturn false
\t\t\t}
\t\t\tcontinue
\t\t}
\t\thasInclude = true
\t\tif hit {
\t\t\tincluded = true
\t\t}
\t}
\treturn !hasInclude || included'''
if old not in s:
    raise SystemExit('MISS')
s = s.replace(old, new, 1)
p.write_text(s, encoding='utf-8')
print('OK ip filter fix')

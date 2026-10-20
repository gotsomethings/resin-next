import pathlib
p = pathlib.Path(r'E:\Develop\resin\internal\probe\manager.go')
s = p.read_text(encoding='utf-8')
bad = '''\t\t// Dynamic throttling: park the task at the front of its lane while the
\t\tlive limit is saturated.'''
good = '''\t\t// Dynamic throttling: park the task at the front of its lane while the
\t\t// live limit is saturated.'''
if bad not in s:
    raise SystemExit('MISS comment')
s = s.replace(bad, good, 1)
p.write_text(s, encoding='utf-8')
print('OK comment fix')

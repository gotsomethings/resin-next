import io, sys, pathlib

def patch(path, old, new, count=1):
    p = pathlib.Path(path)
    s = p.read_text(encoding='utf-8')
    if old not in s:
        print('MISS:', path)
        sys.exit(1)
    if s.count(old) != count:
        print('COUNT:', path, s.count(old))
        sys.exit(1)
    p.write_text(s.replace(old, new), encoding='utf-8')
    print('OK:', path)

patch(r'E:\Develop\resin\internal\state\migrate.go',
'''\t\tstateVersionPlatformRegexFilterRules         = 9
\t\tstateLatestVersion                           = stateVersionPlatformRegexFilterRules''',
'''\t\tstateVersionPlatformRegexFilterRules         = 9
\t\tstateVersionPlatformPolicy                   = 10
\t\tstateLatestVersion                           = stateVersionPlatformPolicy''')

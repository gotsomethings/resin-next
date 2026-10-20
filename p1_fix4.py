import pathlib

p = pathlib.Path(r'E:\Develop\resin\internal\service\control_plane_platform.go')
s = p.read_text(encoding='utf-8')

s = s.replace('func (cfg platformConfig) toRuntime(id string) (*platform.Platform, error) {',
              'func (cfg platformConfig) toRuntime(id string, asnLookup platform.ASNLookupFunc) (*platform.Platform, error) {')

s = s.replace('''\tif s := p.owner(); s != nil {
\t\tplat.ASNLookup = s.asnLookupFunc()
\t}''', '''\tplat.ASNLookup = asnLookup''')

s = s.replace('\tplat, err := cfg.toRuntime(id)\n', '\tplat, err := cfg.toRuntime(id, s.asnLookupFunc())\n')

p.write_text(s, encoding='utf-8')
print('OK toRuntime signature')

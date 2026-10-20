import pathlib

p = pathlib.Path(r'E:\Develop\resin\internal\service\platform_policy_patch.go')
s = p.read_text(encoding='utf-8')
s = s.replace('import (\n\t"time"\n\n\t"github.com/Resinat/Resin/internal/geoip"\n\t"github.com/Resinat/Resin/internal/platform"\n)', 'import (\n\t"net/netip"\n\t"time"\n\n\t"github.com/Resinat/Resin/internal/platform"\n)')
s = s.replace('\nvar _ = geoip.ASNInfo{}\n', '')
p.write_text(s, encoding='utf-8')
print('OK imports fixed')

# control_plane_platform.go imports
p2 = pathlib.Path(r'E:\Develop\resin\internal\service\control_plane_platform.go')
s2 = p2.read_text(encoding='utf-8')
if 'net/netip' not in s2:
    s2 = s2.replace('import (\n\t"encoding/json"', 'import (\n\t"encoding/json"\n\t"net/netip"', 1)
    p2.write_text(s2, encoding='utf-8')
    print('OK netip import')
else:
    print('netip exists')

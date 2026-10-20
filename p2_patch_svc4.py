import pathlib, sys

# 1) ControlPlaneService fields
p = pathlib.Path(r'E:\Develop\resin\internal\service\control_plane_system.go')
s = p.read_text(encoding='utf-8')
old = '''\tProbeMgr        *probe.ProbeManager
'''
new = '''\tProbeMgr        *probe.ProbeManager
\tPlatformProbe   *probe.PlatformProbe
\tVerdicts        *VerdictStore
'''
if old not in s:
    sys.exit('MISS svc fields')
s = s.replace(old, new, 1)
p.write_text(s, encoding='utf-8')
print('OK svc fields')

# 2) toRuntime + call site + blacklist helper + egress snapshot
p2 = pathlib.Path(r'E:\Develop\resin\internal\service\control_plane_platform.go')
s2 = p2.read_text(encoding='utf-8')

s2 = s2.replace('func (cfg platformConfig) toRuntime(id string, asnLookup platform.ASNLookupFunc) (*platform.Platform, error) {',
                'func (cfg platformConfig) toRuntime(id string, asnLookup platform.ASNLookupFunc, blacklist func(netip.Addr) bool) (*platform.Platform, error) {', 1)
s2 = s2.replace('\tplat.ASNLookup = asnLookup\n', '\tplat.ASNLookup = asnLookup\n\tplat.BlacklistLookup = blacklist\n', 1)
s2 = s2.replace('cfg.toRuntime(id, s.asnLookupFunc())', 'cfg.toRuntime(id, s.asnLookupFunc(), s.blacklistLookup(id))', 1)

# egress rows from live store
old = '''\trows, err := s.Engine.ListPlatformEgressIPs(platformID)
\tif err != nil {
\t\treturn nil, internal("list platform egress ips", err)
\t}'''
new = '''\tvar rows []model.PlatformEgressIP
\tif s.Verdicts != nil {
\t\trows = s.Verdicts.Snapshot(platformID)
\t} else {
\t\tvar err error
\t\trows, err = s.Engine.ListPlatformEgressIPs(platformID)
\t\tif err != nil {
\t\t\treturn nil, internal("list platform egress ips", err)
\t\t}
\t}'''
if old not in s2:
    sys.exit('MISS egress rows')
s2 = s2.replace(old, new, 1)

# blacklist helper + lease eviction before geoLookup helper
marker = '// geoLookup adapts the optional GeoIP service.'
helper = '''// blacklistLookup builds the per-platform blacklist predicate.
func (s *ControlPlaneService) blacklistLookup(platformID string) func(netip.Addr) bool {
\tif s == nil || s.Verdicts == nil {
\t\treturn nil
\t}
\treturn func(ip netip.Addr) bool {
\t\treturn s.Verdicts.IsBlacklisted(platformID, ip)
\t}
}

// EvictLeasesForIP drops every lease pinned to the given egress IP.
func (s *ControlPlaneService) EvictLeasesForIP(platformID, ip string) int {
\tleases, err := s.ListLeases(platformID)
\tif err != nil {
\t\treturn 0
\t}
\tevicted := 0
\tfor _, l := range leases {
\t\tif l.EgressIP != ip {
\t\t\tcontinue
\t\t}
\t\tif s.Router.DeleteLease(platformID, l.Account) {
\t\t\tevicted++
\t\t}
\t}
\treturn evicted
}

'''
if marker not in s2:
    sys.exit('MISS geoLookup marker')
s2 = s2.replace(marker, helper + marker, 1)

p2.write_text(s2, encoding='utf-8')
print('OK service wiring')

import pathlib

p = pathlib.Path(r'E:\Develop\resin\internal\platform\platform.go')
s = p.read_text(encoding='utf-8')

if 'type ProbeConfig struct' not in s:
    old = '''// Platform represents a routing platform with its filtered routable view.'''
    new = '''// ProbeConfig is the per-platform probe definition (P2).
type ProbeConfig struct {
\tEnabled         bool
\tURL             string
\tMethod          string
\tHeaders         string
\tBody            string
\tPassStatus      []int
\tFollowRedirects bool
\tInterval        time.Duration
\tScope           string
}

// VerdictConfig is the per-platform verdict/blacklist policy (P2).
type VerdictConfig struct {
\tEnabled               bool
\tThreshold             int
\tRehabInterval         time.Duration
\tRehabSuccessThreshold int
\tEmptyPolicy           string
\tGraceK                int
}

// Platform represents a routing platform with its filtered routable view.'''
    if old not in s:
        raise SystemExit('MISS platform anchor')
    s = s.replace(old, new, 1)

    old2 = '''\t// ASNLookup is optional; nil means "no ASN database loaded".
\tASNLookup ASNLookupFunc'''
    new2 = '''\t// ASNLookup is optional; nil means "no ASN database loaded".
\tASNLookup ASNLookupFunc

\t// Probe/Verdict configuration and the blacklist lookup (P2).
\tProbe           ProbeConfig
\tVerdict         VerdictConfig
\tBlacklistLookup func(netip.Addr) bool'''
    if old2 not in s:
        raise SystemExit('MISS fields anchor')
    s = s.replace(old2, new2, 1)

    old3 = '''\t// 4. Region filter (when configured).'''
    new3 = '''\t// 3b. Per-platform blacklist (verdict state machine).
\tif p.Verdict.Enabled && p.BlacklistLookup != nil && p.BlacklistLookup(egressIP) {
\t\treturn false
\t}

\t// 4. Region filter (when configured).'''
    if old3 not in s:
        raise SystemExit('MISS evaluate anchor')
    s = s.replace(old3, new3, 1)

p.write_text(s, encoding='utf-8')
print('OK platform probe config')

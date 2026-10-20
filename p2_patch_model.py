import pathlib, re

# --- models.go: add P2 fields ---
p = pathlib.Path(r'E:\Develop\resin\internal\model\models.go')
s = p.read_text(encoding='utf-8')

if 'ProbeEnabled' not in s:
    old = '''\tEgressIPFilters      []string
}'''
    new = '''\tEgressIPFilters      []string

\t// --- Platform probe + verdict (P2) ---
\tProbeEnabled          bool   `json:"probe_enabled"`
\tProbeURL              string `json:"probe_url"`
\tProbeMethod           string `json:"probe_method"`
\tProbeHeaders          string `json:"probe_headers"`
\tProbeBody             string `json:"probe_body"`
\tProbePassStatus       []int  `json:"probe_pass_status"`
\tProbeFollowRedirects  bool   `json:"probe_follow_redirects"`
\tProbeIntervalNs       int64  `json:"probe_interval_ns"`
\tProbeScope            string `json:"probe_scope"`
\tVerdictEnabled        bool   `json:"verdict_enabled"`
\tBlacklistThreshold    int    `json:"blacklist_threshold"`
\tRehabIntervalNs       int64  `json:"rehab_interval_ns"`
\tRehabSuccessThreshold int    `json:"rehab_success_threshold"`
\tBlacklistEmptyPolicy  string `json:"blacklist_empty_policy"`
\tBlacklistGraceK       int    `json:"blacklist_grace_k"`
}'''
    if old not in s:
        raise SystemExit('MISS platform fields')
    s = s.replace(old, new, 1)

if 'LastFailAtNs' not in s:
    old = '''\tLastSeenNs      int64  `json:"last_seen_ns"`
}'''
    new = '''\tLastSeenNs      int64  `json:"last_seen_ns"`
\tLastFailAtNs    int64  `json:"last_fail_at_ns"`
\tBlacklistedAtNs int64  `json:"blacklisted_at_ns"`
\tLastNodeHash    string `json:"last_node_hash"`
}'''
    if old not in s:
        raise SystemExit('MISS ledger fields')
    s = s.replace(old, new, 1)

p.write_text(s, encoding='utf-8')
print('OK models')

# --- migrate.go: version 11 ---
p2 = pathlib.Path(r'E:\Develop\resin\internal\state\migrate.go')
s2 = p2.read_text(encoding='utf-8')
s2 = s2.replace('\tstateVersionPlatformPolicy                   = 10\n\tstateLatestVersion                           = stateVersionPlatformPolicy',
                '\tstateVersionPlatformPolicy                   = 10\n\tstateVersionPlatformProbe                    = 11\n\tstateLatestVersion                           = stateVersionPlatformProbe')
p2.write_text(s2, encoding='utf-8')
print('OK migrate version')

import pathlib, sys

p = pathlib.Path(r'E:\Develop\resin\internal\service\control_plane_platform.go')
s = p.read_text(encoding='utf-8')

def rep(old, new):
    global s
    if old not in s:
        print('MISS:', repr(old[:70])); sys.exit(1)
    s = s.replace(old, new, 1)

# 1) response fields
rep('''\tEgressIPFilters      []string `json:"egress_ip_filters"`
}''','''\tEgressIPFilters      []string `json:"egress_ip_filters"`

\t// Probe / verdict surface (P2).
\tProbeEnabled          bool   `json:"probe_enabled"`
\tProbeURL              string `json:"probe_url"`
\tProbeMethod           string `json:"probe_method"`
\tProbeHeaders          string `json:"probe_headers"`
\tProbeBody             string `json:"probe_body"`
\tProbePassStatus       []int  `json:"probe_pass_status"`
\tProbeFollowRedirects  bool   `json:"probe_follow_redirects"`
\tProbeInterval         string `json:"probe_interval"`
\tProbeScope            string `json:"probe_scope"`
\tVerdictEnabled        bool   `json:"verdict_enabled"`
\tBlacklistThreshold    int    `json:"blacklist_threshold"`
\tRehabInterval         string `json:"rehab_interval"`
\tRehabSuccessThreshold int    `json:"rehab_success_threshold"`
\tBlacklistEmptyPolicy  string `json:"blacklist_empty_policy"`
\tBlacklistGraceK       int    `json:"blacklist_grace_k"`
}''')

# 2) platformToResponse fill
rep('''\t\tEgressIPFilters:      append([]string(nil), p.EgressIPFilters...),
\t}''','''\t\tEgressIPFilters:      append([]string(nil), p.EgressIPFilters...),

\t\tProbeEnabled:          p.ProbeEnabled,
\t\tProbeURL:              p.ProbeURL,
\t\tProbeMethod:           p.ProbeMethod,
\t\tProbeHeaders:          p.ProbeHeaders,
\t\tProbeBody:             p.ProbeBody,
\t\tProbePassStatus:       append([]int(nil), p.ProbePassStatus...),
\t\tProbeFollowRedirects:  p.ProbeFollowRedirects,
\t\tProbeInterval:         time.Duration(p.ProbeIntervalNs).String(),
\t\tProbeScope:            p.ProbeScope,
\t\tVerdictEnabled:        p.VerdictEnabled,
\t\tBlacklistThreshold:    p.BlacklistThreshold,
\t\tRehabInterval:         time.Duration(p.RehabIntervalNs).String(),
\t\tRehabSuccessThreshold: p.RehabSuccessThreshold,
\t\tBlacklistEmptyPolicy:  p.BlacklistEmptyPolicy,
\t\tBlacklistGraceK:       p.BlacklistGraceK,
\t}''')

# 3) platformConfig fields
rep('''\t// Policy surface (P1).
\tPolicy platform.Policy
}''','''\t// Policy surface (P1).
\tPolicy platform.Policy

\t// Probe / verdict surface (P2).
\tProbe   platform.ProbeConfig
\tVerdict platform.VerdictConfig
}''')

# 4) fromModel
rep('''\t\tPolicy:                           platform.PolicyFromModel(mp),
\t}''','''\t\tPolicy:                           platform.PolicyFromModel(mp),
\t\tProbe:                            platform.ProbeFromModel(mp),
\t\tVerdict:                          platform.VerdictFromModel(mp),
\t}''')

# 5) toModel
rep('''\tplatform.ApplyPolicyToModel(cfg.Policy, &mp)
\treturn mp''','''\tplatform.ApplyPolicyToModel(cfg.Policy, &mp)
\tplatform.ApplyProbeToModel(cfg.Probe, cfg.Verdict, &mp)
\treturn mp''')

# 6) toRuntime: set probe/verdict + blacklist lookup
rep('''\tplat.ASNLookup = asnLookup
\treturn plat, nil''','''\tplat.ASNLookup = asnLookup
\tplat.Probe = cfg.Probe
\tplat.Verdict = cfg.Verdict
\treturn plat, nil''')

p.write_text(s, encoding='utf-8')
print('OK service probe fields')

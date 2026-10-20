import pathlib, sys

p = pathlib.Path(r'E:\Develop\resin\internal\service\control_plane_platform.go')
s = p.read_text(encoding='utf-8')

def rep(old, new):
    global s
    if old not in s:
        print('MISS:', repr(old[:70])); sys.exit(1)
    s = s.replace(old, new, 1)

# ---- CreatePlatformRequest: probe/verdict ----
rep('''\tEgressIPFilters      []string `json:"egress_ip_filters"`
}

// CreatePlatform creates a new platform.''','''\tEgressIPFilters      []string `json:"egress_ip_filters"`

\tProbeEnabled          *bool   `json:"probe_enabled"`
\tProbeURL              *string `json:"probe_url"`
\tProbeMethod           *string `json:"probe_method"`
\tProbeHeaders          *string `json:"probe_headers"`
\tProbeBody             *string `json:"probe_body"`
\tProbePassStatus       []int   `json:"probe_pass_status"`
\tProbeFollowRedirects  *bool   `json:"probe_follow_redirects"`
\tProbeInterval         *string `json:"probe_interval"`
\tProbeScope            *string `json:"probe_scope"`
\tVerdictEnabled        *bool   `json:"verdict_enabled"`
\tBlacklistThreshold    *int    `json:"blacklist_threshold"`
\tRehabInterval         *string `json:"rehab_interval"`
\tRehabSuccessThreshold *int    `json:"rehab_success_threshold"`
\tBlacklistEmptyPolicy  *string `json:"blacklist_empty_policy"`
\tBlacklistGraceK       *int    `json:"blacklist_grace_k"`
}

// CreatePlatform creates a new platform.''')

# ---- CreatePlatform overlay ----
rep('''\tif svcErr := applyPolicyPatch(&cfg.Policy, policyPatchFromCreate(req)); svcErr != nil {
\t\treturn nil, svcErr
\t}''','''\tif svcErr := applyPolicyPatch(&cfg.Policy, policyPatchFromCreate(req)); svcErr != nil {
\t\treturn nil, svcErr
\t}
\tif svcErr := applyProbePatch(&cfg.Probe, &cfg.Verdict, probePatchFromCreate(req)); svcErr != nil {
\t\treturn nil, svcErr
\t}''')

# ---- UpdatePlatform overlay ----
rep('''\tif svcErr := applyPolicyPatch(&cfg.Policy, policyPatch); svcErr != nil {
\t\treturn nil, svcErr
\t}''','''\tif svcErr := applyPolicyPatch(&cfg.Policy, policyPatch); svcErr != nil {
\t\treturn nil, svcErr
\t}
\tprobePatch, svcErr := probePatchFromMerge(patch)
\tif svcErr != nil {
\t\treturn nil, svcErr
\t}
\tif svcErr := applyProbePatch(&cfg.Probe, &cfg.Verdict, probePatch); svcErr != nil {
\t\treturn nil, svcErr
\t}''')

# ---- validatePlatformConfig: probe/verdict validation ----
rep('''\tif err := platform.Validate(cfg.Policy); err != nil {
\t\treturn invalidArg(err.Error())
\t}
\treturn nil
}''','''\tif err := platform.Validate(cfg.Policy); err != nil {
\t\treturn invalidArg(err.Error())
\t}
\tif err := validateProbeConfig(cfg.Probe); err != nil {
\t\treturn err
\t}
\tif err := validateVerdictConfig(cfg.Verdict); err != nil {
\t\treturn err
\t}
\treturn nil
}''')

p.write_text(s, encoding='utf-8')
print('OK create/update wiring')

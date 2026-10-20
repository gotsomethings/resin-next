import pathlib

# 1) model_codec.go: probe/verdict conversions
p = pathlib.Path(r'E:\Develop\resin\internal\platform\model_codec.go')
s = p.read_text(encoding='utf-8')
if 'func ProbeFromModel' not in s:
    s += '''

// ProbeFromModel extracts the probe configuration with defaults applied.
func ProbeFromModel(mp model.Platform) ProbeConfig {
\tp := ProbeConfig{
\t\tEnabled:         mp.ProbeEnabled,
\t\tURL:             mp.ProbeURL,
\t\tMethod:          strings.ToUpper(strings.TrimSpace(mp.ProbeMethod)),
\t\tHeaders:         mp.ProbeHeaders,
\t\tBody:            mp.ProbeBody,
\t\tPassStatus:      append([]int(nil), mp.ProbePassStatus...),
\t\tFollowRedirects: mp.ProbeFollowRedirects,
\t\tInterval:        time.Duration(mp.ProbeIntervalNs),
\t\tScope:           mp.ProbeScope,
\t}
\tif p.Method == "" {
\t\tp.Method = "POST"
\t}
\tif len(p.PassStatus) == 0 {
\t\tp.PassStatus = []int{200}
\t}
\tif p.Interval <= 0 {
\t\tp.Interval = 30 * time.Minute
\t}
\tif strings.TrimSpace(p.Scope) == "" {
\t\tp.Scope = "untested,failed,blacklisted"
\t}
\treturn p
}

// VerdictFromModel extracts the verdict configuration with defaults applied.
func VerdictFromModel(mp model.Platform) VerdictConfig {
\tv := VerdictConfig{
\t\tEnabled:               mp.VerdictEnabled,
\t\tThreshold:             mp.BlacklistThreshold,
\t\tRehabInterval:         time.Duration(mp.RehabIntervalNs),
\t\tRehabSuccessThreshold: mp.RehabSuccessThreshold,
\t\tEmptyPolicy:           mp.BlacklistEmptyPolicy,
\t\tGraceK:                mp.BlacklistGraceK,
\t}
\tif v.Threshold <= 0 {
\t\tv.Threshold = 3
\t}
\tif v.RehabInterval <= 0 {
\t\tv.RehabInterval = 6 * time.Hour
\t}
\tif v.RehabSuccessThreshold <= 0 {
\t\tv.RehabSuccessThreshold = 3
\t}
\tif v.EmptyPolicy == "" {
\t\tv.EmptyPolicy = "strict"
\t}
\treturn v
}

// ApplyProbeToModel writes probe/verdict config back onto a model platform.
func ApplyProbeToModel(probe ProbeConfig, verdict VerdictConfig, mp *model.Platform) {
\tmp.ProbeEnabled = probe.Enabled
\tmp.ProbeURL = probe.URL
\tmp.ProbeMethod = probe.Method
\tmp.ProbeHeaders = probe.Headers
\tmp.ProbeBody = probe.Body
\tmp.ProbePassStatus = append([]int(nil), probe.PassStatus...)
\tmp.ProbeFollowRedirects = probe.FollowRedirects
\tmp.ProbeIntervalNs = int64(probe.Interval)
\tmp.ProbeScope = probe.Scope
\tmp.VerdictEnabled = verdict.Enabled
\tmp.BlacklistThreshold = verdict.Threshold
\tmp.RehabIntervalNs = int64(verdict.RehabInterval)
\tmp.RehabSuccessThreshold = verdict.RehabSuccessThreshold
\tmp.BlacklistEmptyPolicy = verdict.EmptyPolicy
\tmp.BlacklistGraceK = verdict.GraceK
}
'''
    p.write_text(s, encoding='utf-8')
    print('OK model_codec probe')

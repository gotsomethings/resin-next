import pathlib, sys

p = pathlib.Path(r'E:\Develop\resin\cmd\resin\app_runtime.go')
s = p.read_text(encoding='utf-8')

def rep(old, new):
    global s
    if old not in s:
        print('MISS:', repr(old[:80])); sys.exit(1)
    s = s.replace(old, new, 1)

rep('''\tplatformProbe   *probe.PlatformProbe
\tverdicts        *service.VerdictStore
}''','''\tplatformProbe   *probe.PlatformProbe
\tverdicts        *service.VerdictStore
\tgovernor        *probe.Governor
}''')

start_anchor = '\tlog.Println("Platform probe started")'
if start_anchor not in s:
    print('MISS start anchor'); sys.exit(1)
block = '''\tlog.Println("Platform probe started")

\tgovernor := probe.NewGovernor(probe.GovernorConfig{
\t\tManager: a.topoRuntime.probeMgr,
\t\tOnAdjust: func(effective int, reason string, at time.Time) {
\t\t\tif a.runtimeCfg == nil {
\t\t\t\treturn
\t\t\t}
\t\t\tif cur := a.runtimeCfg.Load(); cur != nil {
\t\t\t\tnext := *cur
\t\t\t\tnext.ProbeConcurrencyEffective = effective
\t\t\t\ta.runtimeCfg.Store(&next)
\t\t\t}
\t\t},
\t\tGC: func() {
\t\t\tif a.verdicts == nil {
\t\t\t\treturn
\t\t\t}
\t\t\tif n, err := a.verdicts.GC(time.Now().Add(-30 * 24 * time.Hour)); err != nil {
\t\t\t\tlog.Printf("verdict GC: %v", err)
\t\t\t} else if n > 0 {
\t\t\t\tlog.Printf("verdict GC: removed %d stale egress IP rows", n)
\t\t\t}
\t\t},
\t})
\tgovernor.Start()
\ta.governor = governor
\tlog.Println("Probe concurrency governor started")'''
s = s.replace(start_anchor, block, 1)

stop_anchor = '''\tif a.platformProbe != nil {
\t\ta.platformProbe.Stop()
\t\tlog.Println("Platform probe stopped")
\t}'''
if stop_anchor not in s:
    print('MISS stop anchor'); sys.exit(1)
stop_block = '''\tif a.governor != nil {
\t\ta.governor.Stop()
\t\tlog.Println("Probe concurrency governor stopped")
\t}
''' + stop_anchor
s = s.replace(stop_anchor, stop_block, 1)

p.write_text(s, encoding='utf-8')
print('OK governor wiring')

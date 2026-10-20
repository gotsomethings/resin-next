import pathlib, sys

p = pathlib.Path(r'E:\Develop\resin\cmd\resin\app_runtime.go')
s = p.read_text(encoding='utf-8')

def rep(old, new):
    global s
    if old not in s:
        print('MISS:', repr(old[:80])); sys.exit(1)
    s = s.replace(old, new, 1)

# app struct: platform probe field
rep('''\ttransportPool   *proxy.OutboundTransportPool
}''','''\ttransportPool   *proxy.OutboundTransportPool
	platformProbe   *probe.PlatformProbe
\tverdicts        *service.VerdictStore
}''')

# wire into cpService
rep('''\tcpService := &service.ControlPlaneService{
\t\tRuntimeCfg:     a.runtimeCfg,''' , '''\tverdictStore := service.NewVerdictStore(engine.StateRepo)
\tif persisted, err := engine.ListPlatforms(); err == nil {
\t\tids := make([]string, 0, len(persisted))
\t\tfor _, pl := range persisted {
\t\t\tids = append(ids, pl.ID)
\t\t}
\t\tif err := verdictStore.Load(ids); err != nil {
\t\t\tlog.Printf("verdict store load: %v", err)
\t\t}
\t}
	a.verdicts = verdictStore

\tcpService := &service.ControlPlaneService{
\t\tVerdicts:       verdictStore,
\t\tRuntimeCfg:     a.runtimeCfg,''')

rep('''\t\tProbeMgr:       a.topoRuntime.probeMgr,
\t\tGeoIP:          a.geoSvc,''' , '''\t\tProbeMgr:       a.topoRuntime.probeMgr,
\t\tGeoIP:          a.geoSvc,''')

# after cpService build: start platform probe (insert before apiSrv :=)
rep('''\tapiSrv := api.NewServerWithAddress(''' , '''\tverdictStore.SetCallbacks(
\t\tfunc(platformID, ip string) {
\t\t\tif n := cpService.EvictLeasesForIP(platformID, ip); n > 0 {
\t\t\t\tlog.Printf("verdict: %s blacklisted %s, evicted %d lease(s)", platformID, ip, n)
\t\t\t}
\t\t},
\t\tfunc(platformID, ip string) {
\t\t\tlog.Printf("verdict: %s rehabilitated %s", platformID, ip)
\t\t},
\t)

\tplatformProbe := probe.NewPlatformProbe(probe.PlatformProbeConfig{
\t\tPool:   a.topoRuntime.pool,
\t\tVerdict: verdictStore,
\t\tTimeout: func() time.Duration { return a.envCfg.ProbeTimeout },
\t\tOnVerdictChange: func(platformID string) {
\t\t\tif plat, ok := a.topoRuntime.pool.GetPlatform(platformID); ok {
\t\t\t\ta.topoRuntime.pool.RebuildPlatform(plat)
\t\t\t}
\t\t},
\t\tTargets: func() []probe.PlatformTarget {
\t\t\tvar out []probe.PlatformTarget
\t\t\ta.topoRuntime.pool.RangePlatforms(func(pl *platform.Platform) bool {
\t\t\t\tout = append(out, probe.PlatformTarget{ID: pl.ID, Probe: pl.Probe, Verdict: pl.Verdict})
\t\t\t\treturn true
\t\t\t})
\t\t\treturn out
\t\t},
\t})
\tcpService.PlatformProbe = platformProbe
\tplatformProbe.Start()
\ta.platformProbe = platformProbe
\tlog.Println("Platform probe started")

\tapiSrv := api.NewServerWithAddress(''')

# shutdown: stop platform probe before ProbeMgr
rep('''\ta.topoRuntime.probeMgr.Stop()
\tlog.Println("Probe manager stopped")''','''\tif a.platformProbe != nil {
\t\ta.platformProbe.Stop()
\t\tlog.Println("Platform probe stopped")
\t}

\ta.topoRuntime.probeMgr.Stop()
\tlog.Println("Probe manager stopped")''')

p.write_text(s, encoding='utf-8')
print('OK app wiring')

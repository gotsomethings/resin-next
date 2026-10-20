import pathlib

p = pathlib.Path(r'E:\Develop\resin\internal\service\platform_verdict_test.go')
s = p.read_text(encoding='utf-8')
old = '''\t// Two passes are not enough.
\tfor i := 1; i <= 2; i++ {
\t\trehab, _ := store.Record("p1", ip, "h1", true, now.Add(time.Duration(10+i)*time.Minute), cfg)
\t\tif rehab {
\t\t\tt.Fatalf("pass %d must not rehabilitate yet", i)
\t\t}
\t}
\trehab, _ := store.Record("p1", ip, "h1", true, now.Add(13*time.Minute), cfg)
\tif !rehab {
\t\tt.Fatal("third consecutive pass must rehabilitate")
\t}
\tif store.IsBlacklisted("p1", netip.MustParseAddr(ip)) {
\t\tt.Fatal("IsBlacklisted must be false after rehab")
\t}
}'''
new = '''\t// First pass only proves the node is answerable -> normal.
\trehab, _ := store.Record("p1", ip, "h1", true, now.Add(10*time.Minute), cfg)
\tif rehab {
\t\tt.Fatal("first pass after blacklist must move to normal, not report rehab")
\t}
\tif store.IsBlacklisted("p1", netip.MustParseAddr(ip)) {
\t\tt.Fatal("first pass after blacklist must clear the blacklist")
\t}
\tif state, _, _ := store.EntryState("p1", ip); state != string(VerdictNormal) {
\t\tt.Fatalf("state after first pass = %s, want normal", state)
\t}

\t// Re-blacklist, then M consecutive passes must rehabilitate.
\tfor i := 0; i < 3; i++ {
\t\tstore.Record("p1", ip, "h1", false, now.Add(time.Duration(20+i)*time.Minute), cfg)
\t}
\tif !store.IsBlacklisted("p1", netip.MustParseAddr(ip)) {
\t\tt.Fatal("expected re-blacklist")
\t}
\tfor i := 1; i <= 2; i++ {
\t\tstore.Record("p1", ip, "h1", true, now.Add(time.Duration(40+i)*time.Minute), cfg)
\t\tif !store.IsBlacklisted("p1", netip.MustParseAddr(ip)) {
\t\t\tt.Fatalf("pass %d of %d must not clear the blacklist", i, 2)
\t\t}
\t}
\trehab, _ = store.Record("p1", ip, "h1", true, now.Add(50*time.Minute), cfg)
\tif !rehab {
\t\tt.Fatalf("%d consecutive passes must rehabilitate", cfg.RehabSuccessThreshold)
\t}
\tif store.IsBlacklisted("p1", netip.MustParseAddr(ip)) {
\t\tt.Fatal("IsBlacklisted must be false after rehab")
\t}
}'''
if old not in s:
    raise SystemExit('MISS test block')
s = s.replace(old, new, 1)
p.write_text(s, encoding='utf-8')
print('OK test rewritten')

import pathlib
p = pathlib.Path(r'E:\Develop\resin\internal\service\platform_verdict.go')
s = p.read_text(encoding='utf-8')
s = s.replace('import (\n\t"net/netip"', 'import (\n\t"fmt"\n\t"net/netip"', 1)
s = s.replace('\t\tswitch VerdictState(e.State) {\n\t\tcase VerdictBlacklisted:', '\t\tfmt.Printf("DBG pass state=%q streak=%d M=%d\\n", e.State, e.SuccessStreak, cfg.RehabSuccessThreshold)\n\t\tswitch VerdictState(e.State) {\n\t\tcase VerdictBlacklisted:', 1)
p.write_text(s, encoding='utf-8')
print('OK dbg2')

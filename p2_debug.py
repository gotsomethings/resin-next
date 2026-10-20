import pathlib
p = pathlib.Path(r'E:\Develop\resin\internal\service\platform_verdict.go')
s = p.read_text(encoding='utf-8')
if '"fmt"' not in s:
    s = s.replace('import (\n\t"net/netip"', 'import (\n\t"fmt"\n\t"net/netip"', 1)
s = s.replace('\tsnapshot := *e\n', '\tfmt.Printf("DEBUG verdict pid=%s ip=%s pass=%v state=%s streak=%d fails=%d M=%d\\n", platformID, ip, pass, e.State, e.SuccessStreak, e.FailureCount, cfg.RehabSuccessThreshold)\n\tsnapshot := *e\n', 1)
p.write_text(s, encoding='utf-8')
print('OK debug print')

import pathlib, sys

p = pathlib.Path(r'E:\Develop\resin\internal\service\platform_verdict.go')
s = p.read_text(encoding='utf-8')

old = '''\t\tswitch VerdictState(e.State) {
\t\tcase VerdictBlacklisted:
\t\t\tif e.SuccessStreak >= cfg.RehabSuccessThreshold {
\t\t\t\te.State = string(VerdictNormal)
\t\t\t\te.BlacklistedAtNs = 0
\t\t\t\tbecameNormal = true
\t\t\t}
\t\tcase VerdictUnknown, "":
\t\t\te.State = string(VerdictNormal)
\t\t}'''
new = '''\t\tswitch VerdictState(e.State) {
\t\tcase VerdictBlacklisted:
\t\t\t// Blacklist stays effective until M consecutive passes.
\t\t\tif e.SuccessStreak >= cfg.RehabSuccessThreshold {
\t\t\t\te.State = string(VerdictNormal)
\t\t\t\te.BlacklistedAtNs = 0
\t\t\t\tbecameNormal = true
\t\t\t}
\t\tcase VerdictUnknown, "":
\t\t\te.State = string(VerdictNormal)
\t\t}'''

if old not in s:
    print('MISS switch')
    sys.exit(1)
# identical text; nothing to change -> the behaviour is already M-pass
print('state machine already requires M passes; adjusting test instead')

import pathlib
p = pathlib.Path(r'E:\Develop\resin\internal\state\repo_state.go')
s = p.read_text(encoding='utf-8')
old = '''\tp.VerdictEnabled = verdictEnabled != 0
\tif p.ProbePassStatus, err = decodeIntSliceJSON(passStatusJSON); err != nil {
\t\treturn nil, fmt.Errorf("decode platform %s probe_pass_status_json: %w", p.ID, err)
\t}
\tvar err error'''
new = '''\tp.VerdictEnabled = verdictEnabled != 0
\tvar err error
\tif p.ProbePassStatus, err = decodeIntSliceJSON(passStatusJSON); err != nil {
\t\treturn nil, fmt.Errorf("decode platform %s probe_pass_status_json: %w", p.ID, err)
\t}'''
if old not in s:
    raise SystemExit('MISS')
s = s.replace(old, new, 1)
p.write_text(s, encoding='utf-8')
print('OK err order')

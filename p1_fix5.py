import pathlib
p = pathlib.Path(r'E:\Develop\resin\internal\service\platform_policy_patch.go')
s = p.read_text(encoding='utf-8')
s = s.replace('\tvar p policyPatch\n\tvar err *ServiceError\n', '\tvar p policyPatch\n', 1)
p.write_text(s, encoding='utf-8')
print('OK unused err removed')

import pathlib
p = pathlib.Path(r'E:\Develop\resin\internal\service\control_plane_platform_preview_test.go')
s = p.read_text(encoding='utf-8')
s = s.replace('\thkEntry.Outbound.Store(&hkOutbound)\n', '\thkEntry.Outbound.Store(&hkOutbound)\n\thkEntry.CircuitOpenSince.Store(0)\n', 1)
s = s.replace('\tusEntry.Outbound.Store(&usOutbound)\n', '\tusEntry.Outbound.Store(&usOutbound)\n\tusEntry.CircuitOpenSince.Store(0)\n', 1)
s = s.replace('\tunknownEntry.Outbound.Store(&unknownOutbound)\n', '\tunknownEntry.Outbound.Store(&unknownOutbound)\n\tunknownEntry.CircuitOpenSince.Store(0)\n', 1)
p.write_text(s, encoding='utf-8')
print('OK circuit reset')

import pathlib

# API handler for GET /platforms/{id}/egress
p = pathlib.Path(r'E:\Develop\resin\internal\api\handler_platform.go')
s = p.read_text(encoding='utf-8')

if 'HandleListPlatformEgress' not in s:
    s += '''
// HandleListPlatformEgress returns a handler for GET /api/v1/platforms/{id}/egress.
func HandleListPlatformEgress(cp *service.ControlPlaneService) http.HandlerFunc {
\treturn func(w http.ResponseWriter, r *http.Request) {
\t\tid, ok := requireUUIDPathParam(w, r, "id", "platform_id")
\t\tif !ok {
\t\t\treturn
\t\t}
\t\trows, err := cp.ListPlatformEgress(id)
\t\tif err != nil {
\t\t\twriteServiceError(w, err)
\t\t\treturn
\t\t}
\t\tWriteJSON(w, http.StatusOK, map[string]any{"items": rows, "total": len(rows)})
\t}
}
'''
    p.write_text(s, encoding='utf-8')
    print('OK egress handler')

# Route registration
p2 = pathlib.Path(r'E:\Develop\resin\internal\api\server.go')
s2 = p2.read_text(encoding='utf-8')
old = '\tauthed.Handle("GET /api/v1/platforms/{id}/ip-load", HandleIPLoad(cp))\n'
new = old + '\tauthed.Handle("GET /api/v1/platforms/{id}/egress", HandleListPlatformEgress(cp))\n'
if old not in s2:
    print('MISS ip-load route')
elif 'platforms/{id}/egress' in s2:
    print('egress route exists')
else:
    s2 = s2.replace(old, new, 1)
    p2.write_text(s2, encoding='utf-8')
    print('OK egress route')

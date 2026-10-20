import pathlib

p = pathlib.Path(r'E:\Develop\resin\internal\service\control_plane_platform.go')
s = p.read_text(encoding='utf-8')

marker = '// geoLookup adapts the optional GeoIP service.'
idx = s.index(marker)

egress = '''// PlatformEgressIPResponse is one ledger row for a platform.
type PlatformEgressIPResponse struct {
\tIP              string `json:"ip"`
\tState           string `json:"state"`
\tFailureCount    int    `json:"failure_count"`
\tSuccessStreak   int    `json:"success_streak"`
\tLastResult      string `json:"last_result"`
\tLastProbeAt     string `json:"last_probe_at,omitempty"`
\tLastSuccessAt   string `json:"last_success_at,omitempty"`
\tFirstSeen       string `json:"first_seen,omitempty"`
\tLastSeen        string `json:"last_seen,omitempty"`
\tNodeCount       int    `json:"node_count"`
}

func formatUnixNs(ns int64) string {
\tif ns <= 0 {
\t\treturn ""
\t}
\treturn time.Unix(0, ns).UTC().Format(time.RFC3339Nano)
}

// ListPlatformEgress returns the per-platform egress-IP ledger joined with the
// current node count per IP. P1: ledger is empty until P2 starts writing it.
func (s *ControlPlaneService) ListPlatformEgress(platformID string) ([]PlatformEgressIPResponse, error) {
\tif _, err := s.getPlatformModel(platformID); err != nil {
\t\treturn nil, err
\t}

\tnodeCounts := make(map[string]int)
\tif s.Pool != nil {
\t\ts.Pool.Range(func(_ node.Hash, entry *node.NodeEntry) bool {
\t\t\tip := entry.GetEgressIP()
\t\t\tif ip.IsValid() {
\t\t\t\tnodeCounts[ip.String()]++
\t\t\t}
\t\t\treturn true
\t\t})
\t}

\trows, err := s.Engine.ListPlatformEgressIPs(platformID)
\tif err != nil {
\t\treturn nil, internal("list platform egress ips", err)
\t}
\tout := make([]PlatformEgressIPResponse, 0, len(rows))
\tfor _, r := range rows {
\t\tout = append(out, PlatformEgressIPResponse{
\t\t\tIP:            r.IP,
\t\t\tState:         r.State,
\t\t\tFailureCount:  r.FailureCount,
\t\t\tSuccessStreak: r.SuccessStreak,
\t\t\tLastResult:    r.LastResult,
\t\t\tLastProbeAt:   formatUnixNs(r.LastProbeAtNs),
\t\t\tLastSuccessAt: formatUnixNs(r.LastSuccessAtNs),
\t\t\tFirstSeen:     formatUnixNs(r.FirstSeenNs),
\t\t\tLastSeen:      formatUnixNs(r.LastSeenNs),
\t\t\tNodeCount:     nodeCounts[r.IP],
\t\t})
\t}
\treturn out, nil
}

'''

s = s[:idx] + egress + s[idx:]
p.write_text(s, encoding='utf-8')
print('OK ListPlatformEgress added')

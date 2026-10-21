package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Resinat/Resin/internal/geoip"
)

// The status endpoint must expose the ASN database state separately from the
// country database, and must never claim ASN availability without a load.
func TestGetGeoIPStatus_ASNFields(t *testing.T) {
	dir := t.TempDir()
	geoSvc := geoip.NewService(geoip.ServiceConfig{CacheDir: dir, OpenDB: geoip.NoOpOpen})
	cp := &ControlPlaneService{GeoIP: geoSvc}

	st := cp.GetGeoIPStatus()
	if st.ASNAvailable {
		t.Fatal("asn_available must be false when no ASN service is attached")
	}
	if st.ASNDBMtime != "" {
		t.Fatalf("asn_db_mtime = %q, want empty", st.ASNDBMtime)
	}

	// A file on disk is reported, but availability still requires a load.
	asnPath := filepath.Join(dir, geoip.DefaultASNFilename)
	if err := os.WriteFile(asnPath, []byte("not-a-real-mmdb"), 0o600); err != nil {
		t.Fatalf("write asn file: %v", err)
	}
	cp.ASN = geoip.NewASNService(asnPath)

	st = cp.GetGeoIPStatus()
	if st.ASNAvailable {
		t.Fatal("an unloaded ASN service must not report availability")
	}
	if st.ASNDBMtime == "" {
		t.Fatal("asn_db_mtime must be reported when the file exists")
	}
	if st.ASNNextUpdate != st.NextScheduledUpdate {
		t.Fatalf("asn_next_update = %q, want %q (same cron entry)", st.ASNNextUpdate, st.NextScheduledUpdate)
	}
}

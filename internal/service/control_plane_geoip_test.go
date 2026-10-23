package service

import (
	"testing"

	"github.com/Resinat/Resin/internal/geoip"
)

// UpdateGeoIPNow must route by target: an unknown target is rejected instead of
// silently refreshing everything, and each known target reaches its own loader.
func TestUpdateGeoIPNow_TargetDispatch(t *testing.T) {
	dir := t.TempDir()
	geoSvc := geoip.NewService(geoip.ServiceConfig{CacheDir: dir, OpenDB: geoip.NoOpOpen})
	// Attach an ASN service so the "asn" target actually reaches the downloader.
	geoSvc.AttachASN(geoip.NewASNService(""))
	cp := &ControlPlaneService{GeoIP: geoSvc}

	if err := cp.UpdateGeoIPNow("bogus"); err == nil {
		t.Fatal("an unknown target must be rejected")
	}

	// No downloader is configured, so every valid target must fail inside the
	// geoip service. A nil error would mean the switch fell through.
	for _, target := range []string{"", "all", "geoip", "asn"} {
		if err := cp.UpdateGeoIPNow(target); err == nil {
			t.Fatalf("target %q: expected a download error with no downloader configured", target)
		}
	}
}

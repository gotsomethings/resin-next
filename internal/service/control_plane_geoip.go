package service

import (
	"net/netip"
	"time"
)

// ------------------------------------------------------------------
// GeoIP
// ------------------------------------------------------------------

// GeoIPStatus is the API response for GeoIP status.
type GeoIPStatus struct {
	DBMtime             string `json:"db_mtime"`
	NextScheduledUpdate string `json:"next_scheduled_update"`
	ASNAvailable        bool   `json:"asn_available"`
	ASNDBMtime          string `json:"asn_db_mtime"`
	ASNNextUpdate       string `json:"asn_next_update"`
}

// GetGeoIPStatus returns the current GeoIP status.
func (s *ControlPlaneService) GetGeoIPStatus() GeoIPStatus {
	status := GeoIPStatus{}
	if s.GeoIP != nil {
		if t := s.GeoIP.LastUpdated(); !t.IsZero() {
			status.DBMtime = t.UTC().Format(time.RFC3339Nano)
		}
		if t := s.GeoIP.NextScheduledUpdate(); !t.IsZero() {
			status.NextScheduledUpdate = t.UTC().Format(time.RFC3339Nano)
			// The ASN database rides the same cron schedule.
			status.ASNNextUpdate = status.NextScheduledUpdate
		}
		if t := s.GeoIP.ASNLastUpdated(); !t.IsZero() {
			status.ASNDBMtime = t.UTC().Format(time.RFC3339Nano)
		}
		if loaded, _ := s.GeoIP.ASNStatus(); loaded {
			status.ASNAvailable = true
		}
	}
	if s.ASN != nil {
		if loaded, _, _ := s.ASN.Status(); loaded {
			status.ASNAvailable = true
		}
	}
	return status
}

// LookupIP performs a GeoIP lookup.
func (s *ControlPlaneService) LookupIP(ipStr string) (string, error) {
	ip, err := netip.ParseAddr(ipStr)
	if err != nil {
		return "", invalidArg("ip: invalid IP address")
	}
	region := s.GeoIP.Lookup(ip)
	return region, nil
}

// UpdateGeoIPNow triggers an immediate database update (blocks). target selects
// which database: "geoip" (country only), "asn" (ASN only) or "all" (both); an
// empty target keeps the historical behaviour and refreshes both.
func (s *ControlPlaneService) UpdateGeoIPNow(target string) error {
	switch target {
	case "", "all":
		if err := s.GeoIP.UpdateAllNow(); err != nil {
			return internal("geoip update failed", err)
		}
	case "geoip":
		if err := s.GeoIP.UpdateNow(); err != nil {
			return internal("geoip update failed", err)
		}
	case "asn":
		if err := s.GeoIP.UpdateASNNow(); err != nil {
			return internal("geoip update failed", err)
		}
	default:
		return invalidArg("target: must be one of geoip|asn|all")
	}
	return nil
}

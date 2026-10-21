package geoip

import (
	"fmt"
	"net"
	"net/netip"
	"os"
	"sync"

	"github.com/oschwald/maxminddb-golang"
)

// ASNInfo is a resolved ASN record for an IP.
type ASNInfo struct {
	Number uint32
	Org    string
}

// ASNReader abstracts a GeoLite2-ASN style database reader.
type ASNReader interface {
	LookupASN(ip netip.Addr) (ASNInfo, bool)
	Close() error
}

type asnMMDBReader struct {
	reader *maxminddb.Reader
}

type asnMMDBRecord struct {
	Number uint32 `maxminddb:"autonomous_system_number"`
	Org    string `maxminddb:"autonomous_system_organization"`
}

func (m *asnMMDBReader) LookupASN(ip netip.Addr) (ASNInfo, bool) {
	if m == nil || m.reader == nil || !ip.IsValid() {
		return ASNInfo{}, false
	}
	ip = ip.Unmap()
	var rec asnMMDBRecord
	if err := m.reader.Lookup(net.IP(ip.AsSlice()), &rec); err != nil {
		return ASNInfo{}, false
	}
	if rec.Number == 0 {
		return ASNInfo{}, false
	}
	return ASNInfo{Number: rec.Number, Org: rec.Org}, true
}

func (m *asnMMDBReader) Close() error {
	if m == nil || m.reader == nil {
		return nil
	}
	return m.reader.Close()
}

// OpenASN opens a MaxMind-compatible ASN mmdb file.
func OpenASN(path string) (ASNReader, error) {
	reader, err := maxminddb.Open(path)
	if err != nil {
		return nil, err
	}
	return &asnMMDBReader{reader: reader}, nil
}

// ASNService holds an optional ASN database. When the file is absent the
// service stays unloaded and every lookup returns ok=false; platform filters
// that require ASN then fail closed.
type ASNService struct {
	mu     sync.RWMutex
	reader ASNReader
	path   string
	err    string
}

// NewASNService creates an ASN service for the given database path.
// An empty path disables ASN lookups.
func NewASNService(path string) *ASNService {
	return &ASNService{path: path}
}

// NewASNServiceWithReader returns an already-loaded service backed by reader.
// Used by tests and by callers that opened the database themselves.
func NewASNServiceWithReader(reader ASNReader) *ASNService {
	return &ASNService{reader: reader}
}

// openASNFile opens the database; swappable in tests.
var openASNFile = OpenASN

// SetPath retargets the service at another database file. The next Load picks
// it up; a failed load keeps the previously loaded reader.
func (s *ASNService) SetPath(path string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.path = path
	s.mu.Unlock()
}

// Path reports the configured database path.
func (s *ASNService) Path() string {
	if s == nil {
		return ""
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.path
}

// Load opens the configured database. It is safe to call again to reload.
func (s *ASNService) Load() error {
	if s == nil {
		return nil
	}
	if s.path == "" {
		s.mu.Lock()
		s.err = "asn_db_not_configured"
		s.mu.Unlock()
		return nil
	}
	if _, statErr := os.Stat(s.path); statErr != nil {
		s.mu.Lock()
		s.err = "asn_db_missing"
		s.mu.Unlock()
		return nil
	}
	reader, err := openASNFile(s.path)
	if err != nil {
		s.mu.Lock()
		s.err = fmt.Sprintf("asn_db_open_failed: %v", err)
		s.mu.Unlock()
		return err
	}
	s.mu.Lock()
	old := s.reader
	s.reader = reader
	s.err = ""
	s.mu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	return nil
}

// Lookup resolves an IP to its ASN record.
func (s *ASNService) Lookup(ip netip.Addr) (ASNInfo, bool) {
	if s == nil {
		return ASNInfo{}, false
	}
	s.mu.RLock()
	r := s.reader
	s.mu.RUnlock()
	if r == nil {
		return ASNInfo{}, false
	}
	return r.LookupASN(ip)
}

// Status reports whether a database is loaded, the configured path, and the
// last load error (if any).
func (s *ASNService) Status() (loaded bool, path string, lastErr string) {
	if s == nil {
		return false, "", "asn_db_not_configured"
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.reader != nil, s.path, s.err
}

// Close releases the database handle.
func (s *ASNService) Close() {
	if s == nil {
		return
	}
	s.mu.Lock()
	r := s.reader
	s.reader = nil
	s.mu.Unlock()
	if r != nil {
		_ = r.Close()
	}
}

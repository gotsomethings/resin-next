package geoip

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mapDownloader serves a fake GitHub release plus the asset bodies.
type mapDownloader struct {
	release map[string]any
	bodies  map[string][]byte
}

func (d *mapDownloader) Download(_ context.Context, url string) ([]byte, error) {
	if url == ReleaseAPIURL {
		return json.Marshal(d.release)
	}
	if body, ok := d.bodies[url]; ok {
		return body, nil
	}
	return nil, fmt.Errorf("unexpected url %q", url)
}

func sha256Digest(body []byte) string {
	sum := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// fakeRelease builds a release document for the given assets and returns it
// together with the download URL of each asset.
func fakeRelease(assets map[string][]byte) (map[string]any, map[string][]byte) {
	list := make([]map[string]any, 0, len(assets))
	bodies := make(map[string][]byte, len(assets))
	for name, body := range assets {
		url := "https://example.test/" + name
		bodies[url] = body
		digest := sha256Digest(body)
		list = append(list, map[string]any{
			"name":                 name,
			"digest":               digest,
			"browser_download_url": url,
		})
	}
	return map[string]any{"tag_name": "v-test", "assets": list}, bodies
}

// stubASNReader stands in for a real mmdb so tests need no binary fixture.
type stubASNReader struct {
	info ASNInfo
	ok   bool
}

func (s stubASNReader) LookupASN(netip.Addr) (ASNInfo, bool) { return s.info, s.ok }
func (s stubASNReader) Close() error                        { return nil }

func stubOpenASN(t *testing.T, info ASNInfo, ok bool) {
	t.Helper()
	orig := openASNFile
	openASNFile = func(string) (ASNReader, error) { return stubASNReader{info: info, ok: ok}, nil }
	t.Cleanup(func() { openASNFile = orig })
}

// One "update now" must install both country.mmdb and GeoLite2-ASN.mmdb from
// the same release, verify their digests, and hot-reload the ASN reader.
func TestASN_UpdateAllNowDownloadsVerifiesAndLoads(t *testing.T) {
	dir := t.TempDir()
	countryDB := []byte("country-mmdb-bytes")
	asnDB := []byte("asn-mmdb-bytes")

	release, bodies := fakeRelease(map[string][]byte{
		"country.mmdb":         countryDB,
		DefaultASNFilename:     asnDB,
	})
	svc := NewService(ServiceConfig{
		CacheDir:   dir,
		Downloader: &mapDownloader{release: release, bodies: bodies},
		OpenDB:     NoOpOpen,
	})
	asnSvc := NewASNService("")
	svc.AttachASN(asnSvc)
	stubOpenASN(t, ASNInfo{Number: 64500, Org: "Test Org"}, true)

	if err := svc.UpdateAllNow(); err != nil {
		t.Fatalf("UpdateAllNow: %v", err)
	}

	for name, want := range map[string][]byte{"country.mmdb": countryDB, DefaultASNFilename: asnDB} {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("%s content = %q, want %q", name, got, want)
		}
	}

	loaded, lastErr := svc.ASNStatus()
	if !loaded {
		t.Fatalf("ASN database not loaded after update: %s", lastErr)
	}

	info, ok := asnSvc.Lookup(netip.MustParseAddr("8.8.8.8"))
	if !ok || info.Number != 64500 || info.Org != "Test Org" {
		t.Fatalf("Lookup = %+v, %v; want 64500/Test Org", info, ok)
	}

	if svc.ASNLastUpdated().IsZero() {
		t.Fatal("ASNLastUpdated must report the installed file mtime")
	}
}

// A release without the ASN asset must fail loudly and leave no file behind.
func TestASN_UpdateMissingAsset(t *testing.T) {
	dir := t.TempDir()
	release, bodies := fakeRelease(map[string][]byte{"country.mmdb": []byte("c")})
	svc := NewService(ServiceConfig{
		CacheDir:   dir,
		Downloader: &mapDownloader{release: release, bodies: bodies},
		OpenDB:     NoOpOpen,
	})
	svc.AttachASN(NewASNService(""))

	err := svc.UpdateASNNow()
	if err == nil {
		t.Fatal("expected an error for a release without the ASN asset")
	}
	if !strings.Contains(err.Error(), DefaultASNFilename) {
		t.Fatalf("error should name the missing asset, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, DefaultASNFilename)); !os.IsNotExist(statErr) {
		t.Fatal("no ASN file must be written when the asset is missing")
	}
}

// Status must be honest while no database file exists.
func TestASN_StatusWithoutDatabase(t *testing.T) {
	dir := t.TempDir()
	svc := NewService(ServiceConfig{CacheDir: dir, OpenDB: NoOpOpen})
	asnSvc := NewASNService(filepath.Join(dir, DefaultASNFilename))
	svc.AttachASN(asnSvc)

	if err := asnSvc.Load(); err != nil {
		t.Fatalf("Load on a missing file must not error: %v", err)
	}

	loaded, lastErr := svc.ASNStatus()
	if loaded {
		t.Fatal("ASN must not report loaded without a database")
	}
	if lastErr != "asn_db_missing" {
		t.Fatalf("lastErr = %q, want asn_db_missing", lastErr)
	}
	if !svc.ASNLastUpdated().IsZero() {
		t.Fatal("ASNLastUpdated must be zero when the file is absent")
	}
}

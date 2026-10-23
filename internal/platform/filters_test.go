package platform

import (
	"net/netip"
	"strings"
	"testing"
)

func TestMatchEgressIPVersion(t *testing.T) {
	v4 := netip.MustParseAddr("1.2.3.4")
	v6 := netip.MustParseAddr("2001:db8::1")

	if !MatchEgressIPVersion(v4, "") || !MatchEgressIPVersion(v6, "") {
		t.Fatal("empty must match both")
	}
	if !MatchEgressIPVersion(v4, "v4") || MatchEgressIPVersion(v6, "v4") {
		t.Fatal("v4 filter wrong")
	}
	if !MatchEgressIPVersion(v6, "v6") || MatchEgressIPVersion(v4, "v6") {
		t.Fatal("v6 filter wrong")
	}
	if err := ValidateEgressIPVersion("v7"); err == nil {
		t.Fatal("v7 must be rejected")
	}
}

func TestMatchEgressIPFilter(t *testing.T) {
	ip := netip.MustParseAddr("1.2.3.4")

	if !MatchEgressIPFilter(ip, nil) {
		t.Fatal("no filters must pass")
	}
	if !MatchEgressIPFilter(ip, []string{"1.2.3.4"}) {
		t.Fatal("exact include must pass")
	}
	if MatchEgressIPFilter(ip, []string{"1.2.3.5"}) {
		t.Fatal("non-matching include must fail")
	}
	if !MatchEgressIPFilter(ip, []string{"1.2.3.0/24"}) {
		t.Fatal("CIDR include must pass")
	}
	if MatchEgressIPFilter(ip, []string{"!1.2.3.0/24"}) {
		t.Fatal("CIDR deny must win")
	}
	if MatchEgressIPFilter(ip, []string{"1.2.3.0/24", "!1.2.3.4"}) {
		t.Fatal("exact deny must win over CIDR include")
	}
	if err := ValidateEgressIPFilters([]string{"not-an-ip"}); err == nil {
		t.Fatal("invalid IP must be rejected")
	}
}

func TestMatchASNFilter(t *testing.T) {
	info := ASNInfo{ASN: 4134, Org: "Chinanet"}
	if !MatchASNFilter(info, true, nil) {
		t.Fatal("empty must pass")
	}
	if !MatchASNFilter(info, true, []string{"as4134"}) {
		t.Fatal("include must pass")
	}
	if MatchASNFilter(info, true, []string{"as12345"}) {
		t.Fatal("other ASN include must fail")
	}
	if MatchASNFilter(info, true, []string{"!4134"}) {
		t.Fatal("deny must fail")
	}
	if MatchASNFilter(info, false, []string{"as4134"}) {
		t.Fatal("unknown ASN with include must fail closed")
	}
	if err := ValidateASNFilters([]string{"asn"}); err == nil {
		t.Fatal("invalid ASN must be rejected")
	}
}

func TestMatchIPTypeFilter(t *testing.T) {
	host := ASNInfo{ASN: 16509, Org: "Amazon.com, Inc."}
	res := ASNInfo{ASN: 4134, Org: "Chinanet"}
	mob := ASNInfo{ASN: 9808, Org: "China Mobile Communications Corporation"}
	unk := ASNInfo{ASN: 64512, Org: "Some Local Network"}

	if ClassifyIPType(host) != IPTypeHosting {
		t.Fatal("amazon must classify as hosting")
	}
	if ClassifyIPType(res) != IPTypeResidential {
		t.Fatal("chinanet must classify as residential")
	}
	if ClassifyIPType(mob) != IPTypeMobile {
		t.Fatal("china mobile must classify as mobile")
	}
	if ClassifyIPType(unk) != IPTypeUnknown {
		t.Fatal("an unrecognised org must classify as unknown, not residential")
	}
	if ClassifyIPType(ASNInfo{}) != IPTypeUnknown {
		t.Fatal("an empty org must classify as unknown")
	}
	if !MatchIPTypeFilter(res, true, []string{"residential"}) {
		t.Fatal("residential include must pass")
	}
	if MatchIPTypeFilter(host, true, []string{"residential"}) {
		t.Fatal("hosting must not pass residential include")
	}
	if !MatchIPTypeFilter(host, true, []string{"!residential"}) {
		t.Fatal("hosting must pass !residential")
	}
	if !MatchIPTypeFilter(unk, true, []string{"unknown"}) {
		t.Fatal("unknown include must pass for an unrecognised org")
	}
	if MatchIPTypeFilter(host, true, []string{"unknown"}) {
		t.Fatal("hosting must not pass an unknown include")
	}
	if !MatchIPTypeFilter(host, true, []string{"!unknown"}) {
		t.Fatal("hosting must pass !unknown")
	}
	if MatchIPTypeFilter(unk, true, []string{"!unknown"}) {
		t.Fatal("an unknown node must not pass !unknown")
	}
	if MatchIPTypeFilter(host, false, []string{"hosting"}) {
		t.Fatal("unknown must fail closed")
	}
	if err := ValidateIPTypeFilters([]string{"mobile"}); err != nil {
		t.Fatalf("mobile must validate: %v", err)
	}
	if err := ValidateIPTypeFilters([]string{"unknown", "!unknown"}); err != nil {
		t.Fatalf("unknown must validate: %v", err)
	}
	if err := ValidateIPTypeFilters([]string{"bogus"}); err == nil {
		t.Fatal("bogus type must be rejected")
	}
}

func TestPolicyValidate(t *testing.T) {
	if err := Validate(Policy{}); err != nil {
		t.Fatalf("zero policy must validate: %v", err)
	}
	if !(Policy{}).IsZero() {
		t.Fatal("zero policy must report IsZero")
	}
	if err := Validate(Policy{MinProbeSuccessRate: 1.5}); err == nil {
		t.Fatal("rate > 1 must be rejected")
	}
	if err := Validate(Policy{LatencyTopN: -1}); err == nil {
		t.Fatal("negative topN must be rejected")
	}
	if (Policy{LatencyTopN: 10}).IsZero() {
		t.Fatal("non-zero policy must not report IsZero")
	}
}

// The egress-country dimension must name itself in the error, not reuse the
// region_filters label (that label pointed operators at the wrong field).
func TestValidateEgressCountryFilters_NamesItsOwnField(t *testing.T) {
	err := ValidateEgressCountryFilters([]string{"US"})
	if err == nil {
		t.Fatal("uppercase country code must be rejected")
	}
	if !strings.Contains(err.Error(), "egress_country_filters[0]") {
		t.Fatalf("error must name egress_country_filters, got: %v", err)
	}
	if strings.Contains(err.Error(), "region_filters") {
		t.Fatalf("error must not mention region_filters, got: %v", err)
	}
	if err := ValidateEgressCountryFilters([]string{"us", "!jp"}); err != nil {
		t.Fatalf("valid country filters must pass: %v", err)
	}
}

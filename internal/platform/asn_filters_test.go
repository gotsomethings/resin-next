package platform

import (
	"net/netip"
	"testing"
)

// stubASN reports ASN 123 for every address.
func stubASN(netip.Addr) (ASNInfo, bool) {
	return ASNInfo{ASN: 123, Org: "Example"}, true
}

// Without an ASN database the ASN / IP-type dimensions cannot be evaluated.
// They must be skipped outright: treating every node as a mismatch would empty
// the platform while the operator believes the filter is being enforced.
func TestFilter_ASNFiltersIgnoredWithoutLookup(t *testing.T) {
	p := NewPlatform("p-asn-none", "ASN none", nil, nil)
	p.Policy = Policy{
		ASNFilters:    []string{"as123"},
		IPTypeFilters: []string{"hosting"},
	}
	if p.ASNLookup != nil {
		t.Fatal("precondition: ASNLookup must be nil")
	}

	h := makeHash(`{"type":"ss","server":"7.7.7.31","port":443}`)
	entry := makeFullyRoutableEntry(h, "sub1")
	rebuildOne(t, p, h, entry)

	if got := p.View().Size(); got != 1 {
		t.Fatalf("ASN filters without a lookup must not filter, got size %d", got)
	}
}

// With a lookup wired the dimensions apply again: a matching ASN passes and a
// negated match is filtered out.
func TestFilter_ASNFiltersApplyWithLookup(t *testing.T) {
	h := makeHash(`{"type":"ss","server":"7.7.7.32","port":443}`)
	entry := makeFullyRoutableEntry(h, "sub1")

	p := NewPlatform("p-asn-yes", "ASN yes", nil, nil)
	p.ASNLookup = stubASN

	p.Policy = Policy{ASNFilters: []string{"as123"}}
	rebuildOne(t, p, h, entry)
	if got := p.View().Size(); got != 1 {
		t.Fatalf("matching ASN must pass, got size %d", got)
	}

	p.Policy = Policy{ASNFilters: []string{"!as123"}}
	rebuildOne(t, p, h, entry)
	if got := p.View().Size(); got != 0 {
		t.Fatalf("negated ASN match must be filtered, got size %d", got)
	}
}

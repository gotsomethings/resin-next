package platform

import (
	"fmt"
	"net/netip"
	"strconv"
	"strings"
)

// ASNInfo is the result of an ASN lookup for an egress IP.
type ASNInfo struct {
	ASN uint32
	Org string
}

// ASNLookupFunc resolves an IP to its ASN/org. ok=false means "not in DB".
type ASNLookupFunc func(netip.Addr) (ASNInfo, bool)

// IP type labels used by ip_type_filters.
const (
	IPTypeResidential = "residential"
	IPTypeHosting     = "hosting"
	IPTypeMobile      = "mobile"
	// IPTypeUnknown is the class for an org name that matches no known keyword
	// (or is empty). Unmatched never guesses a concrete class.
	IPTypeUnknown = "unknown"
)

// hostingKeywords classifies an ASN org as datacenter/hosting.
var hostingKeywords = []string{
	"hosting", "datacenter", "data center", "cloud", "server", "vps",
	"amazon", "aws", "google", "microsoft", "azure", "alibaba", "aliyun",
	"tencent", "digitalocean", "linode", "vultr", "ovh", "hetzner",
	"contabo", "oracle", "leaseweb", "colo", "idc", "m247", "choopa",
}

// mobileKeywords classifies an ASN org as a mobile/cellular carrier.
var mobileKeywords = []string{
	"mobile", "cellular", "wireless", "lte", "4g", "5g",
}

// residentialKeywords classifies an ASN org as a consumer/residential ISP.
var residentialKeywords = []string{
	"chinanet", "china telecom", "china unicom", "telecom", "unicom",
	"comcast", "verizon", "at&t", "charter", "cox", "centurylink",
	"frontier", "spectrum", "bt ", "virgin media", "kddi", "softbank",
	"broadband", "telefonica", "orange", "viettel", "fpt", "vnpt",
	"korea telecom", "sk broadband", "lg u+", "airtel", "jio", "bsnl",
}

// ClassifyIPType derives a coarse IP type from an ASN org name. Keyword
// groups are matched in order: datacenter then mobile then residential. An org
// that matches none of them (or is empty) is IPTypeUnknown, never a guess, so
// a datacenter with an unrecognised name is not reported as residential.
func ClassifyIPType(info ASNInfo) string {
	org := strings.ToLower(strings.TrimSpace(info.Org))
	if org == "" {
		return IPTypeUnknown
	}
	for _, kw := range hostingKeywords {
		if strings.Contains(org, kw) {
			return IPTypeHosting
		}
	}
	for _, kw := range mobileKeywords {
		if strings.Contains(org, kw) {
			return IPTypeMobile
		}
	}
	for _, kw := range residentialKeywords {
		if strings.Contains(org, kw) {
			return IPTypeResidential
		}
	}
	return IPTypeUnknown
}

// --- egress_ip_version ---

// ValidateEgressIPVersion accepts "", "v4" or "v6".
func ValidateEgressIPVersion(v string) error {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "v4", "v6":
		return nil
	default:
		return fmt.Errorf("egress_ip_version: must be \"v4\", \"v6\", or empty, got %q", v)
	}
}

// MatchEgressIPVersion applies the version filter.
func MatchEgressIPVersion(ip netip.Addr, want string) bool {
	want = strings.ToLower(strings.TrimSpace(want))
	if want == "" {
		return true
	}
	if !ip.IsValid() {
		return false
	}
	ip = ip.Unmap()
	switch want {
	case "v4":
		return ip.Is4()
	case "v6":
		return ip.Is6()
	}
	return true
}

// --- asn_filters ---

type asnRule struct {
	asn  uint32
	deny bool
}

// compileASNFilters parses rules like "as123", "!as123", "123", "!123".
func compileASNFilters(filters []string) ([]asnRule, error) {
	out := make([]asnRule, 0, len(filters))
	for i, raw := range filters {
		s := strings.TrimSpace(raw)
		deny := false
		if strings.HasPrefix(s, "!") {
			deny = true
			s = strings.TrimSpace(s[1:])
		}
		s = strings.TrimPrefix(strings.ToLower(s), "as")
		n, err := strconv.ParseUint(s, 10, 32)
		if err != nil || n == 0 {
			return nil, fmt.Errorf("asn_filters[%d]: must be an ASN like as123 or !as123, got %q", i, raw)
		}
		out = append(out, asnRule{asn: uint32(n), deny: deny})
	}
	return out, nil
}

// MatchASNFilter applies include/exclude ASN rules (same polarity as regions).
func MatchASNFilter(info ASNInfo, ok bool, filters []string) bool {
	if len(filters) == 0 {
		return true
	}
	rules, err := compileASNFilters(filters)
	if err != nil {
		return false
	}
	if !ok {
		return false
	}
	hasInclude := false
	included := false
	for _, r := range rules {
		if r.deny {
			if info.ASN == r.asn {
				return false
			}
			continue
		}
		hasInclude = true
		if info.ASN == r.asn {
			included = true
		}
	}
	return !hasInclude || included
}

// --- ip_type_filters ---

// ValidateIPTypeFilters validates residential/hosting/mobile/unknown with an
// optional "!" prefix.
func ValidateIPTypeFilters(filters []string) error {
	for i, raw := range filters {
		s := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(raw, "!")))
		switch s {
		case IPTypeResidential, IPTypeHosting, IPTypeMobile, IPTypeUnknown:
		default:
			return fmt.Errorf("ip_type_filters[%d]: must be residential, hosting, mobile, or unknown (optionally prefixed with !), got %q", i, raw)
		}
	}
	return nil
}

// MatchIPTypeFilter applies include/exclude IP-type rules.
func MatchIPTypeFilter(info ASNInfo, ok bool, filters []string) bool {
	if len(filters) == 0 {
		return true
	}
	if !ok {
		return false
	}
	kind := ClassifyIPType(info)
	included := false
	hasInclude := false
	for _, raw := range filters {
		s := strings.ToLower(strings.TrimSpace(raw))
		deny := strings.HasPrefix(s, "!")
		s = strings.TrimPrefix(s, "!")
		if deny {
			if kind == s {
				return false
			}
			continue
		}
		hasInclude = true
		if kind == s {
			included = true
		}
	}
	return !hasInclude || included
}

// --- egress_ip_filters (manual IP/CIDR allow-deny) ---

// ipRule is one parsed manual egress-IP rule (exact IP or CIDR).
type ipRule struct {
	addr   netip.Addr
	prefix netip.Prefix
	isCIDR bool
	deny   bool
}

// compileIPFilters parses "1.2.3.4", "1.2.3.0/24", with optional "!" denial.
func compileIPFilters(filters []string) ([]ipRule, error) {
	out := make([]ipRule, 0, len(filters))
	for i, raw := range filters {
		s := strings.TrimSpace(raw)
		deny := false
		if strings.HasPrefix(s, "!") {
			deny = true
			s = strings.TrimSpace(s[1:])
		}
		if s == "" {
			return nil, fmt.Errorf("egress_ip_filters[%d]: empty entry", i)
		}
		if strings.Contains(s, "/") {
			p, err := netip.ParsePrefix(s)
			if err != nil {
				return nil, fmt.Errorf("egress_ip_filters[%d]: invalid CIDR %q", i, s)
			}
			out = append(out, ipRule{prefix: p.Masked(), isCIDR: true, deny: deny})
			continue
		}
		a, err := netip.ParseAddr(s)
		if err != nil {
			return nil, fmt.Errorf("egress_ip_filters[%d]: invalid IP %q", i, s)
		}
		out = append(out, ipRule{addr: a.Unmap(), deny: deny})
	}
	return out, nil
}

// MatchEgressIPFilter applies manual include/exclude IP rules.
// Exclusions always win; with no include rules, everything not excluded passes.
func MatchEgressIPFilter(ip netip.Addr, filters []string) bool {
	if len(filters) == 0 {
		return true
	}
	rules, err := compileIPFilters(filters)
	if err != nil {
		return false
	}
	if !ip.IsValid() {
		return false
	}
	ip = ip.Unmap()

	included := false
	hasInclude := false
	for _, r := range rules {
		hit := false
		if r.isCIDR {
			hit = r.prefix.Contains(ip)
		} else {
			hit = r.addr == ip
		}
		if r.deny {
			if hit {
				return false
			}
			continue
		}
		hasInclude = true
		if hit {
			included = true
		}
	}
	return !hasInclude || included
}

// ValidateEgressIPFilters validates manual IP/CIDR filters.
func ValidateEgressIPFilters(filters []string) error {
	if _, err := compileIPFilters(filters); err != nil {
		return err
	}
	return nil
}

// ValidateASNFilters validates asn_filters entries.
func ValidateASNFilters(filters []string) error {
	if _, err := compileASNFilters(filters); err != nil {
		return err
	}
	return nil
}

// ValidateEgressCountryFilters validates country codes on the egress-country
// dimension, naming that field (not region_filters) in the error.
func ValidateEgressCountryFilters(filters []string) error {
	return validateAlpha2List("egress_country_filters", filters)
}

package platform

import (
	"fmt"
	"time"
	"regexp"
	"strings"

	"github.com/Resinat/Resin/internal/model"
	"github.com/Resinat/Resin/internal/node"
)

func isLowerAlpha2(s string) bool {
	if len(s) != 2 {
		return false
	}
	return s[0] >= 'a' && s[0] <= 'z' && s[1] >= 'a' && s[1] <= 'z'
}

// ValidateRegionFilters validates region filters against lowercase ISO alpha-2 format.
// Entries may optionally be prefixed with "!" to indicate negation (e.g. !hk).
func ValidateRegionFilters(regionFilters []string) error {
	for i, r := range regionFilters {
		code := r
		if len(r) > 0 && r[0] == '!' {
			code = r[1:]
		}
		if !isLowerAlpha2(code) {
			return fmt.Errorf("region_filters[%d]: must be a 2-letter lowercase ISO 3166-1 alpha-2 code (e.g. us, jp) or negation (e.g. !hk)", i)
		}
	}
	return nil
}

// CompileRegexFilters compiles line-oriented tag regex rules.
// Plain rules are ANY, "*" prefixes MUST, and "!" prefixes MUST_NOT.
// Only the first byte is interpreted; the remaining regex body is left unchanged.
func CompileRegexFilters(regexFilters []string) (node.TagFilter, error) {
	var compiled node.TagFilter
	for i, rule := range regexFilters {
		pattern := rule
		kind := byte(0)
		if len(rule) > 0 && (rule[0] == '*' || rule[0] == '!') {
			kind = rule[0]
			pattern = rule[1:]
		}

		c, err := regexp.Compile(pattern)
		if err != nil {
			return node.TagFilter{}, fmt.Errorf("regex_filters[%d]: invalid regex: %v", i, err)
		}
		switch kind {
		case '*':
			compiled.Must = append(compiled.Must, c)
		case '!':
			compiled.MustNot = append(compiled.MustNot, c)
		default:
			compiled.Any = append(compiled.Any, c)
		}
	}
	return compiled, nil
}

// NewConfiguredPlatform builds a runtime platform with non-filter settings applied.
func NewConfiguredPlatform(
	id, name string,
	regexFilters node.TagFilter,
	regionFilters []string,
	stickyTTLNs int64,
	missAction string,
	emptyAccountBehavior string,
	fixedAccountHeader string,
	allocationPolicy string,
	passiveCircuitBreakerDisabled bool,
) *Platform {
	return NewConfiguredPlatformWithPolicy(
		id, name, regexFilters, regionFilters, Policy{},
		stickyTTLNs, missAction, emptyAccountBehavior, fixedAccountHeader,
		allocationPolicy, passiveCircuitBreakerDisabled,
	)
}

// NewConfiguredPlatformWithPolicy builds a runtime platform including the
// 11-dimension policy surface.
func NewConfiguredPlatformWithPolicy(
	id, name string,
	regexFilters node.TagFilter,
	regionFilters []string,
	policy Policy,
	stickyTTLNs int64,
	missAction string,
	emptyAccountBehavior string,
	fixedAccountHeader string,
	allocationPolicy string,
	passiveCircuitBreakerDisabled bool,
) *Platform {
	normalizedFixedHeaders, fixedHeaders, err := NormalizeFixedAccountHeaders(fixedAccountHeader)
	if err != nil {
		normalizedFixedHeaders = strings.TrimSpace(fixedAccountHeader)
		fixedHeaders = nil
	}
	plat := NewPlatformWithTagFilter(id, name, regexFilters, regionFilters)
	plat.Policy = policy
	plat.StickyTTLNs = stickyTTLNs
	plat.ReverseProxyMissAction = missAction
	plat.ReverseProxyEmptyAccountBehavior = emptyAccountBehavior
	plat.ReverseProxyFixedAccountHeader = normalizedFixedHeaders
	plat.ReverseProxyFixedAccountHeaders = append([]string(nil), fixedHeaders...)
	plat.AllocationPolicy = ParseAllocationPolicy(allocationPolicy)
	plat.PassiveCircuitBreakerDisabled = passiveCircuitBreakerDisabled
	return plat
}

// CompileModelRegexFilters compiles regex filters from persisted model values.
func CompileModelRegexFilters(platformID string, regexFilters []string) (node.TagFilter, error) {
	compiled, err := CompileRegexFilters(regexFilters)
	if err != nil {
		return node.TagFilter{}, fmt.Errorf("decode platform %s regex_filters: %w", platformID, err)
	}
	return compiled, nil
}

// PolicyFromModel extracts the policy surface from a persisted platform.
func PolicyFromModel(mp model.Platform) Policy {
	return Policy{
		LatencyTopN:          mp.LatencyTopN,
		LatencyMaxMs:         mp.LatencyMaxMs,
		LatencyP95MaxMs:      mp.LatencyP95MaxMs,
		MinAliveStreak:       mp.MinAliveStreak,
		MinProbeSuccessRate:  mp.MinProbeSuccessRate,
		MaxLastProbeAge:      time.Duration(mp.MaxLastProbeAgeNs),
		MinNodeAge:           time.Duration(mp.MinNodeAgeNs),
		EgressIPVersion:      mp.EgressIPVersion,
		EgressCountryFilters: append([]string(nil), mp.EgressCountryFilters...),
		ASNFilters:           append([]string(nil), mp.ASNFilters...),
		IPTypeFilters:        append([]string(nil), mp.IPTypeFilters...),
		EgressIPFilters:      append([]string(nil), mp.EgressIPFilters...),
	}
}

// ApplyPolicyToModel writes the policy surface back onto a model platform.
func ApplyPolicyToModel(p Policy, mp *model.Platform) {
	mp.LatencyTopN = p.LatencyTopN
	mp.LatencyMaxMs = p.LatencyMaxMs
	mp.LatencyP95MaxMs = p.LatencyP95MaxMs
	mp.MinAliveStreak = p.MinAliveStreak
	mp.MinProbeSuccessRate = p.MinProbeSuccessRate
	mp.MaxLastProbeAgeNs = int64(p.MaxLastProbeAge)
	mp.MinNodeAgeNs = int64(p.MinNodeAge)
	mp.EgressIPVersion = p.EgressIPVersion
	mp.EgressCountryFilters = append([]string(nil), p.EgressCountryFilters...)
	mp.ASNFilters = append([]string(nil), p.ASNFilters...)
	mp.IPTypeFilters = append([]string(nil), p.IPTypeFilters...)
	mp.EgressIPFilters = append([]string(nil), p.EgressIPFilters...)
}

// BuildFromModel builds a runtime platform from a persisted model.Platform.
func BuildFromModel(mp model.Platform) (*Platform, error) {
	regexFilters, err := CompileModelRegexFilters(mp.ID, mp.RegexFilters)
	if err != nil {
		return nil, err
	}
	if err := ValidateRegionFilters(mp.RegionFilters); err != nil {
		return nil, err
	}
	policy := PolicyFromModel(mp)
	if err := Validate(policy); err != nil {
		return nil, fmt.Errorf("decode platform %s policy: %w", mp.ID, err)
	}
	emptyAccountBehavior := mp.ReverseProxyEmptyAccountBehavior
	if !ReverseProxyEmptyAccountBehavior(emptyAccountBehavior).IsValid() {
		emptyAccountBehavior = string(ReverseProxyEmptyAccountBehaviorRandom)
	}
	missAction := NormalizeReverseProxyMissAction(mp.ReverseProxyMissAction)
	if missAction == "" {
		return nil, fmt.Errorf(
			"decode platform %s reverse_proxy_miss_action: invalid value %q",
			mp.ID,
			mp.ReverseProxyMissAction,
		)
	}
	fixedHeader, _, err := NormalizeFixedAccountHeaders(mp.ReverseProxyFixedAccountHeader)
	if err != nil {
		return nil, fmt.Errorf("decode platform %s reverse_proxy_fixed_account_header: %w", mp.ID, err)
	}
	if emptyAccountBehavior == string(ReverseProxyEmptyAccountBehaviorFixedHeader) && fixedHeader == "" {
		return nil, fmt.Errorf(
			"decode platform %s reverse_proxy_fixed_account_header: required when reverse_proxy_empty_account_behavior is %s",
			mp.ID,
			ReverseProxyEmptyAccountBehaviorFixedHeader,
		)
	}

	return NewConfiguredPlatformWithPolicy(
		mp.ID,
		mp.Name,
		regexFilters,
		append([]string(nil), mp.RegionFilters...),
		policy,
		mp.StickyTTLNs,
		string(missAction),
		emptyAccountBehavior,
		fixedHeader,
		mp.AllocationPolicy,
		mp.PassiveCircuitBreakerDisabled,
	), nil
}

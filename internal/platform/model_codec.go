package platform

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Resinat/Resin/internal/model"
	"github.com/Resinat/Resin/internal/node"
)

func isLowerAlpha2(s string) bool {
	if len(s) != 2 {
		return false
	}
	return s[0] >= 'a' && s[0] <= 'z' && s[1] >= 'a' && s[1] <= 'z'
}

// validateAlpha2List validates lowercase ISO alpha-2 entries (optionally
// prefixed with "!") and names the offending field in the error, so the
// message matches the request field the caller actually sent.
func validateAlpha2List(field string, list []string) error {
	for i, r := range list {
		code := r
		if len(r) > 0 && r[0] == '!' {
			code = r[1:]
		}
		if !isLowerAlpha2(code) {
			return fmt.Errorf("%s[%d]: must be a 2-letter lowercase ISO 3166-1 alpha-2 code (e.g. us, jp) or negation (e.g. !hk)", field, i)
		}
	}
	return nil
}

// ValidateRegionFilters validates region filters against lowercase ISO alpha-2 format.
// Entries may optionally be prefixed with "!" to indicate negation (e.g. !hk).
func ValidateRegionFilters(regionFilters []string) error {
	return validateAlpha2List("region_filters", regionFilters)
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
	advancedEnabled ...bool,
) *Platform {
	normalizedFixedHeaders, fixedHeaders, err := NormalizeFixedAccountHeaders(fixedAccountHeader)
	if err != nil {
		normalizedFixedHeaders = strings.TrimSpace(fixedAccountHeader)
		fixedHeaders = nil
	}
	plat := NewPlatformWithTagFilter(id, name, regexFilters, regionFilters)
	// Existing callers keep the historical behaviour: advanced filtering is on
	// unless the caller explicitly says otherwise.
	plat.AdvancedEnabled = true
	if len(advancedEnabled) > 0 {
		plat.AdvancedEnabled = advancedEnabled[0]
	}
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
		MinProbeSuccessRateWindow: time.Duration(mp.ProbeRateWindowNs),
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
	mp.ProbeRateWindowNs = int64(p.MinProbeSuccessRateWindow)
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

	plat := NewConfiguredPlatformWithPolicy(
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
		mp.AdvancedEnabled,
	)
	// Probe/verdict/failover must be restored here as well: the platform probe
	// reads Platform.Probe, and a zero value silently disables all platform
	// probing until the platform is next saved from the UI.
	plat.Probe = ProbeFromModel(mp)
	plat.Verdict = VerdictFromModel(mp)
	plat.Failover = FailoverFromModel(mp)
	return plat, nil
}

// ProbeFromModel extracts the probe configuration with defaults applied.
func ProbeFromModel(mp model.Platform) ProbeConfig {
	p := ProbeConfig{
		Enabled:         mp.ProbeEnabled,
		URL:             mp.ProbeURL,
		Method:          strings.ToUpper(strings.TrimSpace(mp.ProbeMethod)),
		Headers:         mp.ProbeHeaders,
		Body:            mp.ProbeBody,
		PassStatus:      append([]int(nil), mp.ProbePassStatus...),
		FollowRedirects: mp.ProbeFollowRedirects,
		Interval:        time.Duration(mp.ProbeIntervalNs),
		Scope:           mp.ProbeScope,
	}
	if p.Method == "" {
		p.Method = "POST"
	}
	if len(p.PassStatus) == 0 {
		p.PassStatus = []int{200}
	}
	if p.Interval <= 0 {
		p.Interval = 30 * time.Minute
	}
	if strings.TrimSpace(p.Scope) == "" {
		p.Scope = "untested,failed,blacklisted"
	}
	return p
}

// VerdictFromModel extracts the verdict configuration with defaults applied.
func VerdictFromModel(mp model.Platform) VerdictConfig {
	v := VerdictConfig{
		Enabled:               mp.VerdictEnabled,
		RehabEnabled:          mp.RehabEnabled,
		Threshold:             mp.BlacklistThreshold,
		RehabInterval:         time.Duration(mp.RehabIntervalNs),
		RehabSuccessThreshold: mp.RehabSuccessThreshold,
	}
	if v.Threshold <= 0 {
		v.Threshold = 3
	}
	if v.RehabInterval <= 0 {
		v.RehabInterval = 6 * time.Hour
	}
	if v.RehabSuccessThreshold <= 0 {
		v.RehabSuccessThreshold = 3
	}
	return v
}

// FailoverFromModel extracts the failover config with defaults applied.
func FailoverFromModel(mp model.Platform) FailoverConfig {
	f := FailoverConfig{
		Enabled:     mp.FailoverEnabled,
		MaxSwitches: mp.FailoverMaxSwitches,
		Deadline:    time.Duration(mp.FailoverDeadlineNs),
		Stage:       mp.FailoverStage,
	}
	if f.MaxSwitches <= 0 {
		f.MaxSwitches = 3
	}
	if f.Deadline <= 0 {
		f.Deadline = 15 * time.Second
	}
	if f.Stage == "" {
		f.Stage = "connect"
	}
	return f
}

// ApplyFailoverToModel writes the failover config back onto a model platform.
func ApplyFailoverToModel(f FailoverConfig, mp *model.Platform) {
	mp.FailoverEnabled = f.Enabled
	mp.FailoverMaxSwitches = f.MaxSwitches
	mp.FailoverDeadlineNs = int64(f.Deadline)
	mp.FailoverStage = f.Stage
}

// ApplyProbeToModel writes probe/verdict config back onto a model platform.
func ApplyProbeToModel(probe ProbeConfig, verdict VerdictConfig, mp *model.Platform) {
	mp.ProbeEnabled = probe.Enabled
	mp.ProbeURL = probe.URL
	mp.ProbeMethod = probe.Method
	mp.ProbeHeaders = probe.Headers
	mp.ProbeBody = probe.Body
	mp.ProbePassStatus = append([]int(nil), probe.PassStatus...)
	mp.ProbeFollowRedirects = probe.FollowRedirects
	mp.ProbeIntervalNs = int64(probe.Interval)
	mp.ProbeScope = probe.Scope
	mp.VerdictEnabled = verdict.Enabled
	mp.RehabEnabled = verdict.RehabEnabled
	mp.BlacklistThreshold = verdict.Threshold
	mp.RehabIntervalNs = int64(verdict.RehabInterval)
	mp.RehabSuccessThreshold = verdict.RehabSuccessThreshold
}

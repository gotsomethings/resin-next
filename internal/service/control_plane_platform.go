package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Resinat/Resin/internal/model"
	"github.com/Resinat/Resin/internal/node"
	"github.com/Resinat/Resin/internal/platform"
	"github.com/Resinat/Resin/internal/routing"
	"github.com/Resinat/Resin/internal/state"
)

// ------------------------------------------------------------------
// Platform
// ------------------------------------------------------------------

// PlatformResponse is the API response model for a platform.
type PlatformResponse struct {
	ID                               string   `json:"id"`
	Name                             string   `json:"name"`
	StickyTTL                        string   `json:"sticky_ttl"`
	RegexFilters                     []string `json:"regex_filters"`
	RegionFilters                    []string `json:"region_filters"`
	RoutableNodeCount                int      `json:"routable_node_count"`
	ReverseProxyMissAction           string   `json:"reverse_proxy_miss_action"`
	ReverseProxyEmptyAccountBehavior string   `json:"reverse_proxy_empty_account_behavior"`
	ReverseProxyFixedAccountHeader   string   `json:"reverse_proxy_fixed_account_header"`
	AllocationPolicy                 string   `json:"allocation_policy"`
	PassiveCircuitBreakerDisabled    bool     `json:"passive_circuit_breaker_disabled"`
	UpdatedAt                        string   `json:"updated_at"`

	// Policy surface (P1).
	LatencyTopN          int      `json:"latency_top_n"`
	LatencyMaxMs         int      `json:"latency_max_ms"`
	LatencyP95MaxMs      int      `json:"latency_p95_max_ms"`
	MinAliveStreak       int      `json:"min_alive_streak"`
	MinProbeSuccessRate  float64  `json:"min_probe_success_rate"`
	ProbeRateWindow      string   `json:"probe_rate_window"`
	MaxLastProbeAge      string   `json:"max_last_probe_age"`
	MinNodeAge           string   `json:"min_node_age"`
	EgressIPVersion      string   `json:"egress_ip_version"`
	EgressCountryFilters []string `json:"egress_country_filters"`
	ASNFilters           []string `json:"asn_filters"`
	IPTypeFilters        []string `json:"ip_type_filters"`
	EgressIPFilters      []string `json:"egress_ip_filters"`

	// Probe / verdict surface (P2).
	ProbeEnabled          bool   `json:"probe_enabled"`
	ProbeURL              string `json:"probe_url"`
	ProbeMethod           string `json:"probe_method"`
	ProbeHeaders          string `json:"probe_headers"`
	ProbeBody             string `json:"probe_body"`
	ProbePassStatus       []int  `json:"probe_pass_status"`
	ProbeFollowRedirects  bool   `json:"probe_follow_redirects"`
	ProbeInterval         string `json:"probe_interval"`
	ProbeScope            string `json:"probe_scope"`
	VerdictEnabled        bool   `json:"verdict_enabled"`
	RehabEnabled          bool   `json:"rehab_enabled"`
	BlacklistThreshold    int    `json:"blacklist_threshold"`
	RehabInterval         string `json:"rehab_interval"`
	RehabSuccessThreshold int    `json:"rehab_success_threshold"`

	// Failover surface (P4).
	FailoverEnabled     bool   `json:"failover_enabled"`
	FailoverMaxSwitches int    `json:"failover_max_switches"`
	FailoverDeadline    string `json:"failover_deadline"`
	FailoverStage       string `json:"failover_stage"`
}

func platformToResponse(p model.Platform) PlatformResponse {
	behavior := normalizePlatformEmptyAccountBehavior(p.ReverseProxyEmptyAccountBehavior)
	fixedHeader := normalizeHeaderFieldName(p.ReverseProxyFixedAccountHeader)
	return PlatformResponse{
		ID:                               p.ID,
		Name:                             p.Name,
		StickyTTL:                        time.Duration(p.StickyTTLNs).String(),
		RegexFilters:                     append([]string(nil), p.RegexFilters...),
		RegionFilters:                    append([]string(nil), p.RegionFilters...),
		RoutableNodeCount:                0,
		ReverseProxyMissAction:           p.ReverseProxyMissAction,
		ReverseProxyEmptyAccountBehavior: behavior,
		ReverseProxyFixedAccountHeader:   fixedHeader,
		AllocationPolicy:                 p.AllocationPolicy,
		PassiveCircuitBreakerDisabled:    p.PassiveCircuitBreakerDisabled,
		UpdatedAt:                        time.Unix(0, p.UpdatedAtNs).UTC().Format(time.RFC3339Nano),

		LatencyTopN:          p.LatencyTopN,
		LatencyMaxMs:         p.LatencyMaxMs,
		LatencyP95MaxMs:      p.LatencyP95MaxMs,
		MinAliveStreak:       p.MinAliveStreak,
		MinProbeSuccessRate:  p.MinProbeSuccessRate,
		ProbeRateWindow:      time.Duration(p.ProbeRateWindowNs).String(),
		MaxLastProbeAge:      time.Duration(p.MaxLastProbeAgeNs).String(),
		MinNodeAge:           time.Duration(p.MinNodeAgeNs).String(),
		EgressIPVersion:      p.EgressIPVersion,
		EgressCountryFilters: append([]string(nil), p.EgressCountryFilters...),
		ASNFilters:           append([]string(nil), p.ASNFilters...),
		IPTypeFilters:        append([]string(nil), p.IPTypeFilters...),
		EgressIPFilters:      append([]string(nil), p.EgressIPFilters...),

		ProbeEnabled:          p.ProbeEnabled,
		ProbeURL:              p.ProbeURL,
		ProbeMethod:           p.ProbeMethod,
		ProbeHeaders:          p.ProbeHeaders,
		ProbeBody:             p.ProbeBody,
		ProbePassStatus:       append([]int(nil), p.ProbePassStatus...),
		ProbeFollowRedirects:  p.ProbeFollowRedirects,
		ProbeInterval:         time.Duration(p.ProbeIntervalNs).String(),
		ProbeScope:            p.ProbeScope,
		VerdictEnabled:        p.VerdictEnabled,
		RehabEnabled:          p.RehabEnabled,
		BlacklistThreshold:    p.BlacklistThreshold,
		RehabInterval:         time.Duration(p.RehabIntervalNs).String(),
		RehabSuccessThreshold: p.RehabSuccessThreshold,

		FailoverEnabled:     p.FailoverEnabled,
		FailoverMaxSwitches: p.FailoverMaxSwitches,
		FailoverDeadline:    time.Duration(p.FailoverDeadlineNs).String(),
		FailoverStage:       p.FailoverStage,
	}
}

func (s *ControlPlaneService) withRoutableNodeCount(resp PlatformResponse) PlatformResponse {
	if s == nil || s.Pool == nil {
		return resp
	}
	plat, ok := s.Pool.GetPlatform(resp.ID)
	if !ok || plat == nil {
		return resp
	}
	resp.RoutableNodeCount = plat.View().Size()
	return resp
}

type platformConfig struct {
	Name                             string
	StickyTTLNs                      int64
	RegexFilters                     []string
	RegionFilters                    []string
	ReverseProxyMissAction           string
	ReverseProxyEmptyAccountBehavior string
	ReverseProxyFixedAccountHeader   string
	AllocationPolicy                 string
	PassiveCircuitBreakerDisabled    bool

	// Policy surface (P1).
	Policy platform.Policy

	// Probe / verdict surface (P2).
	Probe    platform.ProbeConfig
	Verdict  platform.VerdictConfig
	Failover platform.FailoverConfig
}

func normalizePlatformMissAction(raw string) string {
	normalized := platform.NormalizeReverseProxyMissAction(raw)
	if normalized == "" {
		return ""
	}
	return string(normalized)
}

func normalizePlatformEmptyAccountBehavior(raw string) string {
	if platform.ReverseProxyEmptyAccountBehavior(raw).IsValid() {
		return raw
	}
	return string(platform.ReverseProxyEmptyAccountBehaviorRandom)
}

func (s *ControlPlaneService) defaultPlatformConfig(name string) platformConfig {
	return platformConfig{
		Name:                   name,
		StickyTTLNs:            int64(s.EnvCfg.DefaultPlatformStickyTTL),
		RegexFilters:           append([]string(nil), s.EnvCfg.DefaultPlatformRegexFilters...),
		RegionFilters:          append([]string(nil), s.EnvCfg.DefaultPlatformRegionFilters...),
		ReverseProxyMissAction: s.EnvCfg.DefaultPlatformReverseProxyMissAction,
		ReverseProxyEmptyAccountBehavior: normalizePlatformEmptyAccountBehavior(
			s.EnvCfg.DefaultPlatformReverseProxyEmptyAccountBehavior,
		),
		ReverseProxyFixedAccountHeader: normalizeHeaderFieldName(
			s.EnvCfg.DefaultPlatformReverseProxyFixedAccountHeader,
		),
		AllocationPolicy: s.EnvCfg.DefaultPlatformAllocationPolicy,
		Probe:            platform.ProbeFromModel(model.Platform{}),
		Verdict:          platform.VerdictFromModel(model.Platform{}),
		Failover:         platform.FailoverFromModel(model.Platform{}),
	}
}

func platformConfigFromModel(mp model.Platform) platformConfig {
	return platformConfig{
		Name:                             mp.Name,
		StickyTTLNs:                      mp.StickyTTLNs,
		RegexFilters:                     append([]string(nil), mp.RegexFilters...),
		RegionFilters:                    append([]string(nil), mp.RegionFilters...),
		ReverseProxyMissAction:           mp.ReverseProxyMissAction,
		ReverseProxyEmptyAccountBehavior: normalizePlatformEmptyAccountBehavior(mp.ReverseProxyEmptyAccountBehavior),
		ReverseProxyFixedAccountHeader:   normalizeHeaderFieldName(mp.ReverseProxyFixedAccountHeader),
		AllocationPolicy:                 mp.AllocationPolicy,
		PassiveCircuitBreakerDisabled:    mp.PassiveCircuitBreakerDisabled,
		Policy:                           platform.PolicyFromModel(mp),
		Probe:                            platform.ProbeFromModel(mp),
		Verdict:                          platform.VerdictFromModel(mp),
		Failover:                         platform.FailoverFromModel(mp),
	}
}

func (cfg platformConfig) toModel(id string, updatedAtNs int64) model.Platform {
	mp := model.Platform{
		ID:                               id,
		Name:                             cfg.Name,
		StickyTTLNs:                      cfg.StickyTTLNs,
		RegexFilters:                     append([]string(nil), cfg.RegexFilters...),
		RegionFilters:                    append([]string(nil), cfg.RegionFilters...),
		ReverseProxyMissAction:           cfg.ReverseProxyMissAction,
		ReverseProxyEmptyAccountBehavior: cfg.ReverseProxyEmptyAccountBehavior,
		ReverseProxyFixedAccountHeader:   cfg.ReverseProxyFixedAccountHeader,
		AllocationPolicy:                 cfg.AllocationPolicy,
		PassiveCircuitBreakerDisabled:    cfg.PassiveCircuitBreakerDisabled,
		UpdatedAtNs:                      updatedAtNs,
	}
	platform.ApplyPolicyToModel(cfg.Policy, &mp)
	platform.ApplyProbeToModel(cfg.Probe, cfg.Verdict, &mp)
	platform.ApplyFailoverToModel(cfg.Failover, &mp)
	return mp
}

func (cfg platformConfig) toRuntime(id string, asnLookup platform.ASNLookupFunc, blacklist func(netip.Addr) bool) (*platform.Platform, error) {
	compiledRegexFilters, err := platform.CompileRegexFilters(cfg.RegexFilters)
	if err != nil {
		return nil, err
	}
	plat := platform.NewConfiguredPlatformWithPolicy(
		id,
		cfg.Name,
		compiledRegexFilters,
		cfg.RegionFilters,
		cfg.Policy,
		cfg.StickyTTLNs,
		cfg.ReverseProxyMissAction,
		cfg.ReverseProxyEmptyAccountBehavior,
		cfg.ReverseProxyFixedAccountHeader,
		cfg.AllocationPolicy,
		cfg.PassiveCircuitBreakerDisabled,
	)
	plat.ASNLookup = asnLookup
	plat.BlacklistLookup = blacklist
	plat.Probe = cfg.Probe
	plat.Verdict = cfg.Verdict
	plat.Failover = cfg.Failover
	return plat, nil
}

func validatePlatformMissAction(raw string) *ServiceError {
	if normalizePlatformMissAction(raw) != "" {
		return nil
	}
	return invalidArg(fmt.Sprintf(
		"reverse_proxy_miss_action: must be %s or %s",
		platform.ReverseProxyMissActionTreatAsEmpty,
		platform.ReverseProxyMissActionReject,
	))
}

func validatePlatformEmptyAccountBehavior(raw string) *ServiceError {
	if platform.ReverseProxyEmptyAccountBehavior(raw).IsValid() {
		return nil
	}
	return invalidArg(fmt.Sprintf(
		"reverse_proxy_empty_account_behavior: must be %s, %s, or %s",
		platform.ReverseProxyEmptyAccountBehaviorRandom,
		platform.ReverseProxyEmptyAccountBehaviorFixedHeader,
		platform.ReverseProxyEmptyAccountBehaviorAccountHeaderRule,
	))
}

func normalizeHeaderFieldName(raw string) string {
	normalized, _, err := platform.NormalizeFixedAccountHeaders(raw)
	if err != nil {
		return strings.TrimSpace(raw)
	}
	return normalized
}

func validatePlatformEmptyAccountConfig(cfg *platformConfig) *ServiceError {
	if cfg == nil {
		return invalidArg("platform config is required")
	}
	if err := validatePlatformEmptyAccountBehavior(cfg.ReverseProxyEmptyAccountBehavior); err != nil {
		return err
	}
	normalizedFixedHeaders, fixedHeaders, err := platform.NormalizeFixedAccountHeaders(cfg.ReverseProxyFixedAccountHeader)
	if err != nil {
		return invalidArg("reverse_proxy_fixed_account_header: " + err.Error())
	}
	cfg.ReverseProxyFixedAccountHeader = normalizedFixedHeaders
	if cfg.ReverseProxyEmptyAccountBehavior == string(platform.ReverseProxyEmptyAccountBehaviorFixedHeader) &&
		len(fixedHeaders) == 0 {
		return invalidArg(
			"reverse_proxy_fixed_account_header: required when reverse_proxy_empty_account_behavior is FIXED_HEADER",
		)
	}
	return nil
}

func validatePlatformAllocationPolicy(raw string) *ServiceError {
	if platform.AllocationPolicy(raw).IsValid() {
		return nil
	}
	return invalidArg(fmt.Sprintf(
		"allocation_policy: must be %s, %s, or %s",
		platform.AllocationPolicyBalanced,
		platform.AllocationPolicyPreferLowLatency,
		platform.AllocationPolicyPreferIdleIP,
	))
}

func setPlatformStickyTTL(cfg *platformConfig, d time.Duration) *ServiceError {
	if d <= 0 {
		return invalidArg("sticky_ttl: must be > 0")
	}
	cfg.StickyTTLNs = int64(d)
	return nil
}

func setPlatformMissAction(cfg *platformConfig, missAction string) *ServiceError {
	if err := validatePlatformMissAction(missAction); err != nil {
		return err
	}
	cfg.ReverseProxyMissAction = normalizePlatformMissAction(missAction)
	return nil
}

func setPlatformEmptyAccountBehavior(cfg *platformConfig, behavior string) *ServiceError {
	if err := validatePlatformEmptyAccountBehavior(behavior); err != nil {
		return err
	}
	cfg.ReverseProxyEmptyAccountBehavior = behavior
	return nil
}

func setPlatformAllocationPolicy(cfg *platformConfig, policy string) *ServiceError {
	if err := validatePlatformAllocationPolicy(policy); err != nil {
		return err
	}
	cfg.AllocationPolicy = policy
	return nil
}

func validatePlatformConfig(cfg *platformConfig, validateRegionFilters bool) *ServiceError {
	if validateRegionFilters {
		if err := platform.ValidateRegionFilters(cfg.RegionFilters); err != nil {
			return invalidArg(err.Error())
		}
	}
	if err := validatePlatformEmptyAccountConfig(cfg); err != nil {
		return err
	}
	if err := platform.Validate(cfg.Policy); err != nil {
		return invalidArg(err.Error())
	}
	if err := validateProbeConfig(cfg.Probe); err != nil {
		return err
	}
	if err := validateVerdictConfig(cfg.Verdict); err != nil {
		return err
	}
	if err := validateFailoverConfig(cfg.Failover); err != nil {
		return err
	}
	return nil
}

func (s *ControlPlaneService) compileAndUpsertPlatform(id string, cfg platformConfig) (model.Platform, *platform.Platform, *ServiceError) {
	if err := platform.ValidatePlatformName(cfg.Name); err != nil {
		return model.Platform{}, nil, invalidArg("name: " + err.Error())
	}

	plat, err := cfg.toRuntime(id, s.asnLookupFunc(), s.blacklistLookup(id))
	if err != nil {
		return model.Platform{}, nil, invalidArg(err.Error())
	}
	mp := cfg.toModel(id, time.Now().UnixNano())
	if err := s.Engine.UpsertPlatform(mp); err != nil {
		if errors.Is(err, state.ErrConflict) {
			return model.Platform{}, nil, conflict("platform name already exists")
		}
		if strings.HasPrefix(err.Error(), "platform name: ") {
			return model.Platform{}, nil, invalidArg("name: " + strings.TrimPrefix(err.Error(), "platform name: "))
		}
		return model.Platform{}, nil, internal("persist platform", err)
	}
	return mp, plat, nil
}

// ListPlatforms returns all platforms from the database.
func (s *ControlPlaneService) ListPlatforms() ([]PlatformResponse, error) {
	platforms, err := s.Engine.ListPlatforms()
	if err != nil {
		return nil, internal("list platforms", err)
	}
	resp := make([]PlatformResponse, len(platforms))
	for i, p := range platforms {
		resp[i] = s.withRoutableNodeCount(platformToResponse(p))
	}
	return resp, nil
}

func (s *ControlPlaneService) getPlatformModel(id string) (*model.Platform, error) {
	p, err := s.Engine.GetPlatform(id)
	if err != nil {
		if errors.Is(err, state.ErrNotFound) {
			return nil, notFound("platform not found")
		}
		return nil, internal("get platform", err)
	}
	return p, nil
}

// GetPlatform returns a single platform by ID.
func (s *ControlPlaneService) GetPlatform(id string) (*PlatformResponse, error) {
	mp, err := s.getPlatformModel(id)
	if err != nil {
		return nil, err
	}
	r := s.withRoutableNodeCount(platformToResponse(*mp))
	return &r, nil
}

// CreatePlatformRequest holds create platform parameters.
type CreatePlatformRequest struct {
	Name                             *string  `json:"name"`
	StickyTTL                        *string  `json:"sticky_ttl"`
	RegexFilters                     []string `json:"regex_filters"`
	RegionFilters                    []string `json:"region_filters"`
	ReverseProxyMissAction           *string  `json:"reverse_proxy_miss_action"`
	ReverseProxyEmptyAccountBehavior *string  `json:"reverse_proxy_empty_account_behavior"`
	ReverseProxyFixedAccountHeader   *string  `json:"reverse_proxy_fixed_account_header"`
	AllocationPolicy                 *string  `json:"allocation_policy"`
	PassiveCircuitBreakerDisabled    *bool    `json:"passive_circuit_breaker_disabled"`

	LatencyTopN          *int     `json:"latency_top_n"`
	LatencyMaxMs         *int     `json:"latency_max_ms"`
	LatencyP95MaxMs      *int     `json:"latency_p95_max_ms"`
	MinAliveStreak       *int     `json:"min_alive_streak"`
	MinProbeSuccessRate  *float64 `json:"min_probe_success_rate"`
	ProbeRateWindow      *string  `json:"probe_rate_window"`
	MaxLastProbeAge      *string  `json:"max_last_probe_age"`
	MinNodeAge           *string  `json:"min_node_age"`
	EgressIPVersion      *string  `json:"egress_ip_version"`
	EgressCountryFilters []string `json:"egress_country_filters"`
	ASNFilters           []string `json:"asn_filters"`
	IPTypeFilters        []string `json:"ip_type_filters"`
	EgressIPFilters      []string `json:"egress_ip_filters"`

	ProbeEnabled          *bool   `json:"probe_enabled"`
	ProbeURL              *string `json:"probe_url"`
	ProbeMethod           *string `json:"probe_method"`
	ProbeHeaders          *string `json:"probe_headers"`
	ProbeBody             *string `json:"probe_body"`
	ProbePassStatus       []int   `json:"probe_pass_status"`
	ProbeFollowRedirects  *bool   `json:"probe_follow_redirects"`
	ProbeInterval         *string `json:"probe_interval"`
	ProbeScope            *string `json:"probe_scope"`
	VerdictEnabled        *bool   `json:"verdict_enabled"`
	RehabEnabled          *bool   `json:"rehab_enabled"`
	BlacklistThreshold    *int    `json:"blacklist_threshold"`
	RehabInterval         *string `json:"rehab_interval"`
	RehabSuccessThreshold *int    `json:"rehab_success_threshold"`

	FailoverEnabled     *bool   `json:"failover_enabled"`
	FailoverMaxSwitches *int    `json:"failover_max_switches"`
	FailoverDeadline    *string `json:"failover_deadline"`
	FailoverStage       *string `json:"failover_stage"`
}

// CreatePlatform creates a new platform.
func (s *ControlPlaneService) CreatePlatform(req CreatePlatformRequest) (*PlatformResponse, error) {
	// Validate name.
	if req.Name == nil {
		return nil, invalidArg("name is required")
	}
	name := platform.NormalizePlatformName(*req.Name)
	if name == "" {
		return nil, invalidArg("name is required")
	}
	if err := platform.ValidatePlatformName(name); err != nil {
		return nil, invalidArg("name: " + err.Error())
	}
	if name == platform.DefaultPlatformName {
		return nil, conflict("cannot use reserved name 'Default'")
	}

	// Apply defaults from env and overlay request fields.
	cfg := s.defaultPlatformConfig(name)
	if req.StickyTTL != nil {
		d, err := time.ParseDuration(*req.StickyTTL)
		if err != nil {
			return nil, invalidArg("sticky_ttl: " + err.Error())
		}
		if err := setPlatformStickyTTL(&cfg, d); err != nil {
			return nil, err
		}
	}
	if req.RegexFilters != nil {
		cfg.RegexFilters = req.RegexFilters
	}
	if req.RegionFilters != nil {
		cfg.RegionFilters = req.RegionFilters
	}
	if req.ReverseProxyMissAction != nil {
		if err := setPlatformMissAction(&cfg, *req.ReverseProxyMissAction); err != nil {
			return nil, err
		}
	}
	if req.ReverseProxyEmptyAccountBehavior != nil {
		if err := setPlatformEmptyAccountBehavior(&cfg, *req.ReverseProxyEmptyAccountBehavior); err != nil {
			return nil, err
		}
	}
	if req.ReverseProxyFixedAccountHeader != nil {
		cfg.ReverseProxyFixedAccountHeader = *req.ReverseProxyFixedAccountHeader
	}
	if req.AllocationPolicy != nil {
		if err := setPlatformAllocationPolicy(&cfg, *req.AllocationPolicy); err != nil {
			return nil, err
		}
	}
	if req.PassiveCircuitBreakerDisabled != nil {
		cfg.PassiveCircuitBreakerDisabled = *req.PassiveCircuitBreakerDisabled
	}
	if svcErr := applyPolicyPatch(&cfg.Policy, policyPatchFromCreate(req)); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := applyProbePatch(&cfg.Probe, &cfg.Verdict, probePatchFromCreate(req)); svcErr != nil {
		return nil, svcErr
	}
	applyFailoverPatch(&cfg.Failover, failoverPatchFromCreate(req))
	if err := validatePlatformConfig(&cfg, true); err != nil {
		return nil, err
	}

	id := uuid.New().String()
	mp, plat, svcErr := s.compileAndUpsertPlatform(id, cfg)
	if svcErr != nil {
		return nil, svcErr
	}

	// Register in topology pool.
	// Build the routable view before publish so concurrent readers don't observe
	// a newly created platform with an empty view.
	s.Pool.RebuildPlatform(plat)
	s.Pool.RegisterPlatform(plat)

	r := s.withRoutableNodeCount(platformToResponse(mp))
	return &r, nil
}

// UpdatePlatform applies a constrained partial patch to a platform.
// This is not RFC 7396 JSON Merge Patch: patch must be a non-empty object and
// null values are rejected.
func (s *ControlPlaneService) UpdatePlatform(id string, patchJSON json.RawMessage) (*PlatformResponse, error) {
	patch, verr := parseMergePatch(patchJSON)
	if verr != nil {
		return nil, verr
	}
	if err := patch.validateFields(platformPatchAllowedFields, func(key string) string {
		return fmt.Sprintf("field %q is read-only or unknown", key)
	}); err != nil {
		return nil, err
	}

	// Load current.
	current, err := s.getPlatformModel(id)
	if err != nil {
		return nil, err
	}
	if current.ID == platform.DefaultPlatformID {
		if nameVal, ok := patch["name"]; ok {
			nameStr, _ := nameVal.(string)
			if nameStr != platform.DefaultPlatformName {
				return nil, conflict("cannot rename Default platform")
			}
		}
	}

	cfg := platformConfigFromModel(*current)

	// Apply patch to current config.
	if nameStr, ok, err := patch.optionalNonEmptyString("name"); err != nil {
		return nil, err
	} else if ok {
		cfg.Name = platform.NormalizePlatformName(nameStr)
		if err := platform.ValidatePlatformName(cfg.Name); err != nil {
			return nil, invalidArg("name: " + err.Error())
		}
		if cfg.Name == platform.DefaultPlatformName && current.ID != platform.DefaultPlatformID {
			return nil, conflict("cannot use reserved name 'Default'")
		}
	}

	if d, ok, err := patch.optionalDurationString("sticky_ttl"); err != nil {
		return nil, err
	} else if ok {
		if err := setPlatformStickyTTL(&cfg, d); err != nil {
			return nil, err
		}
	}

	if filters, ok, err := patch.optionalStringSlice("regex_filters"); err != nil {
		return nil, err
	} else if ok {
		cfg.RegexFilters = filters
	}

	regionFiltersPatched := false
	if filters, ok, err := patch.optionalStringSlice("region_filters"); err != nil {
		return nil, err
	} else if ok {
		regionFiltersPatched = true
		cfg.RegionFilters = filters
	}

	if ma, ok, err := patch.optionalString("reverse_proxy_miss_action"); err != nil {
		return nil, err
	} else if ok {
		if err := setPlatformMissAction(&cfg, ma); err != nil {
			return nil, err
		}
	}
	if behavior, ok, err := patch.optionalString("reverse_proxy_empty_account_behavior"); err != nil {
		return nil, err
	} else if ok {
		if err := setPlatformEmptyAccountBehavior(&cfg, behavior); err != nil {
			return nil, err
		}
	}
	if fixedHeader, ok, err := patch.optionalString("reverse_proxy_fixed_account_header"); err != nil {
		return nil, err
	} else if ok {
		cfg.ReverseProxyFixedAccountHeader = fixedHeader
	}

	if ap, ok, err := patch.optionalString("allocation_policy"); err != nil {
		return nil, err
	} else if ok {
		if err := setPlatformAllocationPolicy(&cfg, ap); err != nil {
			return nil, err
		}
	}
	if disabled, ok, err := patch.optionalBool("passive_circuit_breaker_disabled"); err != nil {
		return nil, err
	} else if ok {
		cfg.PassiveCircuitBreakerDisabled = disabled
	}
	policyPatch, svcErr := policyPatchFromMerge(patch)
	if svcErr != nil {
		return nil, svcErr
	}
	if svcErr := applyPolicyPatch(&cfg.Policy, policyPatch); svcErr != nil {
		return nil, svcErr
	}
	probePatch, svcErr := probePatchFromMerge(patch)
	if svcErr != nil {
		return nil, svcErr
	}
	if svcErr := applyProbePatch(&cfg.Probe, &cfg.Verdict, probePatch); svcErr != nil {
		return nil, svcErr
	}
	failoverPatch, svcErr := failoverPatchFromMerge(patch)
	if svcErr != nil {
		return nil, svcErr
	}
	applyFailoverPatch(&cfg.Failover, failoverPatch)
	if err := validatePlatformConfig(&cfg, regionFiltersPatched); err != nil {
		return nil, err
	}

	mp, plat, svcErr := s.compileAndUpsertPlatform(id, cfg)
	if svcErr != nil {
		return nil, svcErr
	}

	// Replace in topology pool.
	if err := s.Pool.ReplacePlatform(plat); err != nil {
		return nil, internal("replace platform in pool", err)
	}

	r := s.withRoutableNodeCount(platformToResponse(mp))
	return &r, nil
}

// DeletePlatform deletes a platform.
func (s *ControlPlaneService) DeletePlatform(id string) error {
	if id == platform.DefaultPlatformID {
		return conflict("cannot delete Default platform")
	}

	if err := s.Engine.DeletePlatform(id); err != nil {
		if errors.Is(err, state.ErrNotFound) {
			return notFound("platform not found")
		}
		return internal("delete platform", err)
	}
	s.Pool.UnregisterPlatform(id)
	return nil
}

// ResetPlatformToDefault resets a platform to env defaults.
func (s *ControlPlaneService) ResetPlatformToDefault(id string) (*PlatformResponse, error) {
	name, err := s.Engine.GetPlatformName(id)
	if err != nil {
		if errors.Is(err, state.ErrNotFound) {
			return nil, notFound("platform not found")
		}
		return nil, internal("get platform", err)
	}

	cfg := s.defaultPlatformConfig(name)
	mp, plat, svcErr := s.compileAndUpsertPlatform(id, cfg)
	if svcErr != nil {
		return nil, svcErr
	}

	if err := s.Pool.ReplacePlatform(plat); err != nil {
		return nil, internal("replace platform in pool", err)
	}

	r := s.withRoutableNodeCount(platformToResponse(mp))
	return &r, nil
}

// RebuildPlatformView triggers a full rebuild of the platform's routable view.
func (s *ControlPlaneService) RebuildPlatformView(id string) error {
	plat, ok := s.Pool.GetPlatform(id)
	if !ok {
		return notFound("platform not found")
	}
	s.Pool.RebuildPlatform(plat)
	return nil
}

// PreviewFilterRequest holds preview filter parameters.
type PreviewFilterRequest struct {
	PlatformID   *string             `json:"platform_id"`
	PlatformSpec *PlatformSpecFilter `json:"platform_spec"`
}

type PlatformSpecFilter struct {
	RegexFilters  []string `json:"regex_filters"`
	RegionFilters []string `json:"region_filters"`

	LatencyTopN          int      `json:"latency_top_n"`
	LatencyMaxMs         int      `json:"latency_max_ms"`
	LatencyP95MaxMs      int      `json:"latency_p95_max_ms"`
	MinAliveStreak       int      `json:"min_alive_streak"`
	MinProbeSuccessRate  float64  `json:"min_probe_success_rate"`
	ProbeRateWindow      string   `json:"probe_rate_window"`
	MaxLastProbeAge      string   `json:"max_last_probe_age"`
	MinNodeAge           string   `json:"min_node_age"`
	EgressIPVersion      string   `json:"egress_ip_version"`
	EgressCountryFilters []string `json:"egress_country_filters"`
	ASNFilters           []string `json:"asn_filters"`
	IPTypeFilters        []string `json:"ip_type_filters"`
	EgressIPFilters      []string `json:"egress_ip_filters"`
}

// policy converts the preview spec into a platform policy.
func (f PlatformSpecFilter) policy() platform.Policy {
	p := platform.Policy{
		LatencyTopN:          f.LatencyTopN,
		LatencyMaxMs:         f.LatencyMaxMs,
		LatencyP95MaxMs:      f.LatencyP95MaxMs,
		MinAliveStreak:       f.MinAliveStreak,
		MinProbeSuccessRate:  f.MinProbeSuccessRate,
		EgressIPVersion:      f.EgressIPVersion,
		EgressCountryFilters: append([]string(nil), f.EgressCountryFilters...),
		ASNFilters:           append([]string(nil), f.ASNFilters...),
		IPTypeFilters:        append([]string(nil), f.IPTypeFilters...),
		EgressIPFilters:      append([]string(nil), f.EgressIPFilters...),
	}
	if f.MaxLastProbeAge != "" {
		if d, err := time.ParseDuration(f.MaxLastProbeAge); err == nil {
			p.MaxLastProbeAge = d
		}
	}
	if f.MinNodeAge != "" {
		if d, err := time.ParseDuration(f.MinNodeAge); err == nil {
			p.MinNodeAge = d
		}
	}
	return p
}

// NodeSummary is the API response for a node.
type NodeSummary struct {
	NodeHash                         string   `json:"node_hash"`
	CreatedAt                        string   `json:"created_at"`
	Enabled                          bool     `json:"enabled"`
	DisplayTag                       string   `json:"display_tag,omitempty"`
	HasOutbound                      bool     `json:"has_outbound"`
	LastError                        string   `json:"last_error,omitempty"`
	CircuitOpenSince                 *string  `json:"circuit_open_since"`
	FailureCount                     int      `json:"failure_count"`
	EgressIP                         string   `json:"egress_ip,omitempty"`
	Region                           string   `json:"region,omitempty"`
	LastEgressUpdate                 string   `json:"last_egress_update,omitempty"`
	LastLatencyProbeAttempt          string   `json:"last_latency_probe_attempt,omitempty"`
	LastAuthorityLatencyProbeAttempt string   `json:"last_authority_latency_probe_attempt,omitempty"`
	ReferenceLatencyMs               *float64 `json:"reference_latency_ms,omitempty"`
	LastEgressUpdateAttempt          string   `json:"last_egress_update_attempt,omitempty"`
	ASNNumber                        uint32   `json:"asn_number"`
	ASNOrg                           string   `json:"asn_org,omitempty"`
	// IPType is the coarse class derived from the ASN org (residential/hosting).
	IPType string `json:"ip_type,omitempty"`
	// PendingTest is true while the node has never been probed: no egress IP
	// yet, or no probe attempt recorded.
	PendingTest bool      `json:"pending_test"`
	Tags        []NodeTag `json:"tags"`
}

// IsHealthyAndEnabled follows the node-summary health rule used by API/UI
// aggregates: enabled, outbound-ready, and not circuit-open.
func (n NodeSummary) IsHealthyAndEnabled() bool {
	return n.Enabled && n.HasOutbound && n.CircuitOpenSince == nil
}

type NodeTag struct {
	SubscriptionID          string `json:"subscription_id"`
	SubscriptionName        string `json:"subscription_name"`
	Tag                     string `json:"tag"`
	SubscriptionCreatedAtNs int64  `json:"-"`
}

func (s *ControlPlaneService) nodeEntryToSummary(h node.Hash, entry *node.NodeEntry) NodeSummary {
	ns := NodeSummary{
		NodeHash:     h.Hex(),
		CreatedAt:    entry.CreatedAt.UTC().Format(time.RFC3339Nano),
		Enabled:      true,
		HasOutbound:  entry.HasOutbound(),
		LastError:    entry.GetLastError(),
		FailureCount: int(entry.FailureCount.Load()),
	}

	if s != nil && s.Pool != nil {
		ns.Enabled = !s.Pool.IsNodeDisabled(h)
		ns.DisplayTag = s.Pool.ResolveNodeDisplayTag(h)
	}

	if cos := entry.CircuitOpenSince.Load(); cos > 0 {
		t := time.Unix(0, cos).UTC().Format(time.RFC3339Nano)
		ns.CircuitOpenSince = &t
	}

	egressIP := entry.GetEgressIP()
	if egressIP.IsValid() {
		ns.EgressIP = egressIP.String()
		ns.Region = entry.GetRegion(nil)
		if s.GeoIP != nil {
			ns.Region = entry.GetRegion(s.GeoIP.Lookup)
		}
	}
	// ASN columns: zero/empty when no database is attached or the IP is absent.
	if info, ok := s.nodeASN(entry); ok {
		ns.ASNNumber = info.Number
		ns.ASNOrg = info.Org
	}
	ns.IPType = s.nodeIPType(entry)
	ns.PendingTest = nodeIsPendingTest(entry)

	if leu := entry.LastEgressUpdate.Load(); leu > 0 {
		ns.LastEgressUpdate = time.Unix(0, leu).UTC().Format(time.RFC3339Nano)
	}
	if lastAny := entry.LastLatencyProbeAttempt.Load(); lastAny > 0 {
		ns.LastLatencyProbeAttempt = time.Unix(0, lastAny).UTC().Format(time.RFC3339Nano)
	}
	if lastAuthority := entry.LastAuthorityLatencyProbeAttempt.Load(); lastAuthority > 0 {
		ns.LastAuthorityLatencyProbeAttempt = time.Unix(0, lastAuthority).UTC().Format(time.RFC3339Nano)
	}
	if s != nil && s.RuntimeCfg != nil {
		if cfg := s.RuntimeCfg.Load(); cfg != nil {
			if avgMs, ok := node.AverageEWMAForDomainsMs(entry, cfg.LatencyAuthorities); ok {
				ns.ReferenceLatencyMs = &avgMs
			}
		}
	}
	if lastEgressAttempt := entry.LastEgressUpdateAttempt.Load(); lastEgressAttempt > 0 {
		ns.LastEgressUpdateAttempt = time.Unix(0, lastEgressAttempt).UTC().Format(time.RFC3339Nano)
	}

	// Build tags.
	subIDs := entry.SubscriptionIDs()
	for _, subID := range subIDs {
		sub := s.SubMgr.Lookup(subID)
		if sub == nil {
			continue
		}
		managed, ok := sub.ManagedNodes().LoadNode(h)
		if !ok {
			continue
		}
		tags := managed.Tags
		for _, tag := range tags {
			ns.Tags = append(ns.Tags, NodeTag{
				SubscriptionID:          subID,
				SubscriptionName:        sub.Name(),
				Tag:                     sub.Name() + "/" + tag,
				SubscriptionCreatedAtNs: sub.CreatedAtNs,
			})
		}
	}
	if ns.Tags == nil {
		ns.Tags = []NodeTag{}
	}
	return ns
}

// PreviewFilter returns nodes matching the given filter spec.
func (s *ControlPlaneService) PreviewFilter(req PreviewFilterRequest) ([]NodeSummary, error) {
	hasPlatformID := req.PlatformID != nil && *req.PlatformID != ""
	hasPlatformSpec := req.PlatformSpec != nil

	if hasPlatformID == hasPlatformSpec {
		return nil, invalidArg("exactly one of platform_id or platform_spec is required")
	}

	var regexFilters node.TagFilter
	var regionFilters []string
	var policy platform.Policy

	if hasPlatformID {
		plat, ok := s.Pool.GetPlatform(*req.PlatformID)
		if !ok {
			return nil, notFound("platform not found")
		}
		regexFilters = plat.RegexFilters
		regionFilters = append([]string(nil), plat.RegionFilters...)
		policy = plat.Policy
	} else {
		compiled, err := platform.CompileRegexFilters(req.PlatformSpec.RegexFilters)
		if err != nil {
			return nil, invalidArg(err.Error())
		}
		regexFilters = compiled
		regionFilters = req.PlatformSpec.RegionFilters
		if err := platform.ValidateRegionFilters(regionFilters); err != nil {
			return nil, invalidArg(err.Error())
		}
		policy = req.PlatformSpec.policy()
		if err := platform.Validate(policy); err != nil {
			return nil, invalidArg(err.Error())
		}
	}

	tmp := platform.NewConfiguredPlatformWithPolicy(
		"preview", "preview", regexFilters, regionFilters, policy,
		0, string(platform.ReverseProxyMissActionTreatAsEmpty),
		string(platform.ReverseProxyEmptyAccountBehaviorRandom), "", "BALANCED", false,
	)
	tmp.ASNLookup = s.asnLookupFunc()

	subLookup := s.Pool.MakeSubLookup()
	poolRange := func(fn func(node.Hash, *node.NodeEntry) bool) {
		s.Pool.Range(fn)
	}
	tmp.FullRebuild(poolRange, subLookup, s.geoLookup())

	var result []NodeSummary
	tmp.View().Range(func(h node.Hash) bool {
		entry, ok := s.Pool.GetEntry(h)
		if !ok {
			return true
		}
		result = append(result, s.nodeEntryToSummary(h, entry))
		return true
	})
	return result, nil
}

// PlatformEgressIPResponse is one row of the platform IP list: the egress-IP
// ledger joined with the nodes currently passing the platform configuration.
type PlatformEgressIPResponse struct {
	IP              string   `json:"ip"`
	Country         string   `json:"country,omitempty"`
	IPType          string   `json:"ip_type,omitempty"`
	State           string   `json:"state"`
	NodeCount       int      `json:"node_count"`
	InUse           bool     `json:"in_use"`
	LeaseCount      int      `json:"lease_count"`
	FailureCount    int      `json:"failure_count"`
	SuccessStreak   int      `json:"success_streak"`
	LastStatusCode  int      `json:"last_status_code"`
	LastResult      string   `json:"last_result"`
	JoinedAt        string   `json:"joined_at,omitempty"`
	LastProbeAt     string   `json:"last_probe_at,omitempty"`
	LastSuccessAt   string   `json:"last_success_at,omitempty"`
	SuccessRate     *float64 `json:"success_rate,omitempty"`
	ManualBlacklist bool     `json:"manual_blacklist"`
}

func formatUnixNs(ns int64) string {
	if ns <= 0 {
		return ""
	}
	return time.Unix(0, ns).UTC().Format(time.RFC3339Nano)
}

// ipAgg accumulates the per-egress-IP facts across the nodes behind that IP.
type ipAgg struct {
	count   int
	country string
	ipType  string
	joined  int64
	succ    int
	total   int
}

// ListPlatformEgress returns the platform IP list: the platform's own IP library,
// i.e. every node that is healthy and passes the platform's filters (base config +
// advanced dimensions, including latency, probe rate and probe age), joined with
// the verdict ledger.
//
// The verdict blacklist is not a filter here: a blacklisted IP stays listed with
// its state while Resin still regards its nodes as healthy, so a rehab probe or a
// manual release can revive it without the row disappearing in between.
func (s *ControlPlaneService) ListPlatformEgress(platformID string) ([]PlatformEgressIPResponse, error) {
	if _, err := s.getPlatformModel(platformID); err != nil {
		return nil, err
	}

	now := time.Now()
	var window time.Duration
	aggs := make(map[string]*ipAgg)

	if s.Pool != nil {
		if rt, ok := s.Pool.GetPlatform(platformID); ok && rt != nil {
			window = rt.Policy.MinProbeSuccessRateWindow
			geo := s.geoLookup()
			asn := s.asnLookupFunc()
			subLookup := s.Pool.MakeSubLookup()
			// The list is the platform's IP library: the platform's own filters (base
			// config + advanced dimensions, including probe rate and probe age) decide
			// membership, and the node must be healthy. The verdict blacklist is not a
			// filter here: a blacklisted IP stays listed with its state so it can be
			// revived by a rehab probe or released manually.
			s.Pool.Range(func(_ node.Hash, entry *node.NodeEntry) bool {
				if !rt.EvaluateForList(entry, subLookup, geo) {
					return true
				}
				ip := entry.GetEgressIP()
				if !ip.IsValid() {
					return true
				}
				key := ip.Unmap().String()
				a := aggs[key]
				if a == nil {
					a = &ipAgg{}
					if geo != nil {
						a.country = geo(ip)
					}
					if asn != nil {
						if info, ok := asn(ip); ok {
							a.ipType = platform.ClassifyIPType(info)
						}
					}
					aggs[key] = a
				}
				a.count++
				if created := entry.CreatedAt.UnixNano(); created > 0 && (a.joined == 0 || created < a.joined) {
					a.joined = created
				}
				succ, total := entry.ProbeWindowStats(now, window)
				a.succ += succ
				a.total += total
				return true
			})
		}
	}

	ledger := make(map[string]model.PlatformEgressIP)
	if s.Verdicts != nil {
		for _, r := range s.Verdicts.Snapshot(platformID) {
			ledger[r.IP] = r
		}
	} else if s.Engine != nil {
		rows, err := s.Engine.ListPlatformEgressIPs(platformID)
		if err != nil {
			return nil, internal("list platform egress ips", err)
		}
		for _, r := range rows {
			ledger[r.IP] = r
		}
	}

	// Only IPs that currently have nodes inside the advanced pool are listed: an
	// IP without nodes cannot be used, so a stale ledger state must not make it
	// look available.
	leaseCounts := make(map[string]int)
	if s.Router != nil && s.Pool != nil {
		if _, ok := s.Pool.GetPlatform(platformID); ok {
			nowNs := now.UnixNano()
			s.Router.RangeLeases(platformID, func(_ string, lease routing.Lease) bool {
				if lease.ExpiryNs > 0 && lease.ExpiryNs <= nowNs {
					return true
				}
				leaseCounts[lease.EgressIP.String()]++
				return true
			})
		}
	}

	out := make([]PlatformEgressIPResponse, 0, len(aggs))
	for ip, a := range aggs {
		row := PlatformEgressIPResponse{
			IP:        ip,
			Country:   a.country,
			IPType:    a.ipType,
			NodeCount: a.count,
			State:     string(VerdictUnknown),
		}
		row.LeaseCount = leaseCounts[ip]
		row.InUse = row.LeaseCount > 0
		if r, ok := ledger[ip]; ok {
			row.FailureCount = r.FailureCount
			row.SuccessStreak = r.SuccessStreak
			row.LastStatusCode = r.LastStatusCode
			row.LastResult = r.LastResult
			row.LastProbeAt = formatUnixNs(r.LastProbeAtNs)
			row.LastSuccessAt = formatUnixNs(r.LastSuccessAtNs)
			row.ManualBlacklist = r.ManualBlacklist
			row.JoinedAt = formatUnixNs(r.FirstSeenNs)
			switch {
			case r.State == string(VerdictBlacklisted):
				row.State = string(VerdictBlacklisted)
			case r.LastProbeAtNs > 0 && r.LastResult == "pass":
				row.State = string(VerdictNormal)
			case r.LastProbeAtNs > 0:
				row.State = "failed"
			}
		}
		if row.JoinedAt == "" && a.joined > 0 {
			row.JoinedAt = formatUnixNs(a.joined)
		}
		if a.total > 0 {
			rate := float64(a.succ) / float64(a.total)
			row.SuccessRate = &rate
		}
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].IP < out[j].IP })
	return out, nil
}

// SetPlatformEgressBlacklist manually blacklists (on=true) or releases an
// egress IP for one platform and reports whether the state changed. A manual
// blacklist is never cleared by the automatic rehab streak.
func (s *ControlPlaneService) SetPlatformEgressBlacklist(platformID, ipStr string, blacklisted bool) (bool, error) {
	if _, err := s.getPlatformModel(platformID); err != nil {
		return false, err
	}
	ip, err := netip.ParseAddr(strings.TrimSpace(ipStr))
	if err != nil {
		return false, invalidArg("ip: invalid format")
	}
	if s.Verdicts == nil {
		return false, internal("verdict store unavailable", nil)
	}
	ip = ip.Unmap()
	if !s.Verdicts.SetManualBlacklist(platformID, ip.String(), blacklisted, time.Now()) {
		return false, nil
	}
	if blacklisted {
		s.EvictLeasesForIP(platformID, ip.String())
	}
	return true, nil
}

// blacklistLookup builds the per-platform blacklist predicate.
func (s *ControlPlaneService) blacklistLookup(platformID string) func(netip.Addr) bool {
	if s == nil || s.Verdicts == nil {
		return nil
	}
	return func(ip netip.Addr) bool {
		return s.Verdicts.IsBlacklisted(platformID, ip)
	}
}

// EvictLeasesForIP drops every lease pinned to the given egress IP.
func (s *ControlPlaneService) EvictLeasesForIP(platformID, ip string) int {
	leases, err := s.ListLeases(platformID)
	if err != nil {
		return 0
	}
	evicted := 0
	for _, l := range leases {
		if l.EgressIP != ip {
			continue
		}
		if s.Router.DeleteLease(platformID, l.Account) {
			evicted++
		}
	}
	return evicted
}

// geoLookup adapts the optional GeoIP service.
func (s *ControlPlaneService) geoLookup() platform.GeoLookupFunc {
	if s == nil || s.GeoIP == nil {
		return nil
	}
	return s.GeoIP.Lookup
}

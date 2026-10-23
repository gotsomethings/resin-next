// Package model defines domain structs shared across the persistence layer.
package model

import "encoding/json"

// Platform represents a routing platform.
type Platform struct {
	ID                               string `json:"id"`
	Name                             string `json:"name"`
	StickyTTLNs                      int64  `json:"sticky_ttl_ns"`
	RegexFilters                     []string
	RegionFilters                    []string
	ReverseProxyMissAction           string `json:"reverse_proxy_miss_action"`
	ReverseProxyEmptyAccountBehavior string `json:"reverse_proxy_empty_account_behavior"`
	ReverseProxyFixedAccountHeader   string `json:"reverse_proxy_fixed_account_header"`
	AllocationPolicy                 string `json:"allocation_policy"`
	PassiveCircuitBreakerDisabled    bool   `json:"passive_circuit_breaker_disabled"`
	UpdatedAtNs                      int64  `json:"updated_at_ns"`

	// --- Platform policy (11-dimension filter surface, P1) ---
	LatencyTopN          int     `json:"latency_top_n"`
	LatencyMaxMs         int     `json:"latency_max_ms"`
	LatencyP95MaxMs      int     `json:"latency_p95_max_ms"`
	MinAliveStreak       int     `json:"min_alive_streak"`
	MinProbeSuccessRate  float64 `json:"min_probe_success_rate"`
	MaxLastProbeAgeNs    int64   `json:"max_last_probe_age_ns"`
	MinNodeAgeNs         int64   `json:"min_node_age_ns"`
	EgressIPVersion      string  `json:"egress_ip_version"`
	EgressCountryFilters []string
	ASNFilters           []string
	IPTypeFilters        []string
	EgressIPFilters      []string

	// --- Platform probe + verdict (P2) ---
	ProbeEnabled          bool   `json:"probe_enabled"`
	ProbeURL              string `json:"probe_url"`
	ProbeMethod           string `json:"probe_method"`
	ProbeHeaders          string `json:"probe_headers"`
	ProbeBody             string `json:"probe_body"`
	ProbePassStatus       []int  `json:"probe_pass_status"`
	ProbeFollowRedirects  bool   `json:"probe_follow_redirects"`
	ProbeIntervalNs       int64  `json:"probe_interval_ns"`
	ProbeScope            string `json:"probe_scope"`
	VerdictEnabled        bool   `json:"verdict_enabled"`
	RehabEnabled          bool   `json:"rehab_enabled"`
	BlacklistThreshold    int    `json:"blacklist_threshold"`
	RehabIntervalNs       int64  `json:"rehab_interval_ns"`
	RehabSuccessThreshold int    `json:"rehab_success_threshold"`

	// --- Failover (P4) ---
	FailoverEnabled     bool   `json:"failover_enabled"`
	FailoverMaxSwitches int    `json:"failover_max_switches"`
	FailoverDeadlineNs  int64  `json:"failover_deadline_ns"`
	FailoverStage       string `json:"failover_stage"`
}

// PlatformEgressIP is one row of the per-platform egress-IP ledger.
// P1 exposes it read-only; P2 (verdict state machine) writes it.
type PlatformEgressIP struct {
	PlatformID      string `json:"platform_id"`
	IP              string `json:"ip"`
	State           string `json:"state"`
	FailureCount    int    `json:"failure_count"`
	SuccessStreak   int    `json:"success_streak"`
	LastResult      string `json:"last_result"`
	LastProbeAtNs   int64  `json:"last_probe_at_ns"`
	LastSuccessAtNs int64  `json:"last_success_at_ns"`
	FirstSeenNs     int64  `json:"first_seen_ns"`
	LastSeenNs      int64  `json:"last_seen_ns"`
	LastFailAtNs    int64  `json:"last_fail_at_ns"`
	BlacklistedAtNs int64  `json:"blacklisted_at_ns"`
	LastNodeHash    string `json:"last_node_hash"`
}

// Subscription represents a node subscription source.
type Subscription struct {
	ID                        string `json:"id"`
	Name                      string `json:"name"`
	SourceType                string `json:"source_type"`
	URL                       string `json:"url"`
	Content                   string `json:"content"`
	UpdateIntervalNs          int64  `json:"update_interval_ns"`
	Enabled                   bool   `json:"enabled"`
	Ephemeral                 bool   `json:"ephemeral"`
	IncrementalAliveNodes     bool   `json:"incremental_alive_nodes"`
	EphemeralNodeEvictDelayNs int64  `json:"ephemeral_node_evict_delay_ns"`
	CreatedAtNs               int64  `json:"created_at_ns"`
	UpdatedAtNs               int64  `json:"updated_at_ns"`
}

// Endpoint represents a persisted custom inbound listener. The environment-
// defined default endpoint is synthesized at runtime and is never stored here.
type Endpoint struct {
	ID                   string `json:"id"`
	Port                 int    `json:"port"`
	Enabled              bool   `json:"enabled"`
	AllowManagement      bool   `json:"allow_management"`
	AllowProxy           bool   `json:"allow_proxy"`
	RequireProxyAuthInfo bool   `json:"require_proxy_auth_info"`
	AllowHTTPForward     bool   `json:"allow_http_forward"`
	AllowHTTPReverse     bool   `json:"allow_http_reverse"`
	AllowSOCKS5          bool   `json:"allow_socks5"`
	CreatedAtNs          int64  `json:"created_at_ns"`
	UpdatedAtNs          int64  `json:"updated_at_ns"`
}

// AccountHeaderRule defines header extraction rules for reverse proxy account matching.
type AccountHeaderRule struct {
	URLPrefix   string `json:"url_prefix"`
	Headers     []string
	UpdatedAtNs int64 `json:"updated_at_ns"`
}

// NodeStatic holds the immutable portion of a node's data.
type NodeStatic struct {
	Hash        string          `json:"hash"`
	RawOptions  json.RawMessage `json:"raw_options_json"`
	CreatedAtNs int64           `json:"created_at_ns"`
}

// NodeDynamic holds the mutable runtime state of a node.
type NodeDynamic struct {
	Hash                               string `json:"hash"`
	FailureCount                       int    `json:"failure_count"`
	CircuitOpenSince                   int64  `json:"circuit_open_since"`
	EgressIP                           string `json:"egress_ip"`
	EgressRegion                       string `json:"egress_region"`
	EgressUpdatedAtNs                  int64  `json:"egress_updated_at_ns"`
	LastLatencyProbeAttemptNs          int64  `json:"last_latency_probe_attempt_ns"`
	LastAuthorityLatencyProbeAttemptNs int64  `json:"last_authority_latency_probe_attempt_ns"`
	LastEgressUpdateAttemptNs          int64  `json:"last_egress_update_attempt_ns"`
}

// NodeLatency holds per-domain latency statistics for a node.
type NodeLatency struct {
	NodeHash      string `json:"node_hash"`
	Domain        string `json:"domain"`
	EwmaNs        int64  `json:"ewma_ns"`
	LastUpdatedNs int64  `json:"last_updated_ns"`
}

// NodeLatencyKey is the composite primary key for node_latency.
type NodeLatencyKey struct {
	NodeHash string
	Domain   string
}

// Lease represents a sticky routing lease.
type Lease struct {
	PlatformID     string `json:"platform_id"`
	Account        string `json:"account"`
	NodeHash       string `json:"node_hash"`
	EgressIP       string `json:"egress_ip"`
	CreatedAtNs    int64  `json:"created_at_ns"`
	ExpiryNs       int64  `json:"expiry_ns"`
	LastAccessedNs int64  `json:"last_accessed_ns"`
}

// LeaseKey is the composite primary key for leases.
type LeaseKey struct {
	PlatformID string
	Account    string
}

// SubscriptionNode links a subscription to a node with tags.
type SubscriptionNode struct {
	SubscriptionID string `json:"subscription_id"`
	NodeHash       string `json:"node_hash"`
	Tags           []string
	Evicted        bool `json:"evicted"`
}

// SubscriptionNodeKey is the composite primary key for subscription_nodes.
type SubscriptionNodeKey struct {
	SubscriptionID string
	NodeHash       string
}

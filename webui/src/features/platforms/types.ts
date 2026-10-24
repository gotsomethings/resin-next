export type PlatformMissAction = "TREAT_AS_EMPTY" | "REJECT";
export type PlatformEmptyAccountBehavior = "RANDOM" | "FIXED_HEADER" | "ACCOUNT_HEADER_RULE";
export type PlatformAllocationPolicy = "BALANCED" | "PREFER_LOW_LATENCY" | "PREFER_IDLE_IP";

export type Platform = {
  id: string;
  name: string;
  sticky_ttl: string;
  regex_filters: string[];
  region_filters: string[];
  routable_node_count: number;
  reverse_proxy_miss_action: PlatformMissAction;
  reverse_proxy_empty_account_behavior: PlatformEmptyAccountBehavior;
  reverse_proxy_fixed_account_header: string;
  allocation_policy: PlatformAllocationPolicy;
  passive_circuit_breaker_disabled: boolean;
  updated_at: string;
};

/** P1 filter surface. */
export type PlatformPolicyFields = {
  latency_top_n: number;
  latency_max_ms: number;
  latency_p95_max_ms: number;
  min_alive_streak: number;
  min_probe_success_rate: number;
  probe_rate_window: string;
  max_last_probe_age: string;
  min_node_age: string;
  egress_ip_version: string;
  egress_country_filters: string[];
  asn_filters: string[];
  ip_type_filters: string[];
  egress_ip_filters: string[];
};

/** P2 probe/verdict surface. */
export type PlatformProbeFields = {
  probe_enabled: boolean;
  probe_url: string;
  probe_method: string;
  probe_headers: string;
  probe_body: string;
  probe_pass_status: number[];
  probe_follow_redirects: boolean;
  probe_interval: string;
  probe_scope: string;
  verdict_enabled: boolean;
  rehab_enabled: boolean;
  blacklist_threshold: number;
  rehab_interval: string;
  rehab_success_threshold: number;
};

/** P4 failover surface. */
export type PlatformFailoverFields = {
  failover_enabled: boolean;
  failover_max_switches: number;
  failover_deadline: string;
  failover_stage: string;
};

export type PlatformExtras = PlatformPolicyFields & PlatformProbeFields & PlatformFailoverFields;

export type PageResponse<T> = {
  items: T[];
  total: number;
  limit: number;
  offset: number;
};

export type PlatformCreateInput = {
  name: string;
  sticky_ttl?: string;
  regex_filters?: string[];
  region_filters?: string[];
  reverse_proxy_miss_action?: PlatformMissAction;
  reverse_proxy_empty_account_behavior?: PlatformEmptyAccountBehavior;
  reverse_proxy_fixed_account_header?: string;
  allocation_policy?: PlatformAllocationPolicy;
  passive_circuit_breaker_disabled?: boolean;
};

export type PlatformUpdateInput = {
  name?: string;
  sticky_ttl?: string;
  regex_filters?: string[];
  region_filters?: string[];
  reverse_proxy_miss_action?: PlatformMissAction;
  reverse_proxy_empty_account_behavior?: PlatformEmptyAccountBehavior;
  reverse_proxy_fixed_account_header?: string;
  allocation_policy?: PlatformAllocationPolicy;
  passive_circuit_breaker_disabled?: boolean;
} & Partial<PlatformPolicyFields> &
  Partial<PlatformProbeFields> &
  Partial<PlatformFailoverFields>;

export type PlatformLease = {
  platform_id: string;
  account: string;
  node_hash: string;
  node_tag: string;
  egress_ip: string;
  expiry: string;
  last_accessed: string;
};

export type PlatformLeaseSortBy = "account" | "expiry" | "last_accessed";
export type SortOrder = "asc" | "desc";

export type ListPlatformLeasesInput = {
  limit?: number;
  offset?: number;
  account?: string;
  fuzzy?: boolean;
  sort_by?: PlatformLeaseSortBy;
  sort_order?: SortOrder;
};


export type EgressIPState = "unknown" | "normal" | "failed" | "blacklisted" | "";
export type EgressIPType = "residential" | "hosting" | "mobile" | "unknown" | "";

/** One row of the platform IP list: config-filtered egress IP + verdict. */
export type EgressIPRow = {
  ip: string;
  country?: string;
  ip_type?: EgressIPType;
  state: EgressIPState;
  node_count: number;
  base_node_count: number;
  in_base_pool: boolean;
  in_advanced_pool: boolean;
  failure_count: number;
  success_streak: number;
  last_status_code: number;
  last_result: string;
  joined_at?: string;
  last_probe_at?: string;
  last_success_at?: string;
  success_rate?: number | null;
  manual_blacklist?: boolean;
};

export type EgressIPPage = { items: EgressIPRow[]; total: number };


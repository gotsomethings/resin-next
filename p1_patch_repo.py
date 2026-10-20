import pathlib, sys

p = pathlib.Path(r'E:\Develop\resin\internal\state\repo_state.go')
s = p.read_text(encoding='utf-8')
start = s.index('// --- platforms ---')
end = s.index('// --- subscriptions ---')

block = '''// --- platforms ---

// platformColumns is the full column list used by Get/List.
const platformColumns = `id, name, sticky_ttl_ns, regex_filters_json, region_filters_json,
\treverse_proxy_miss_action, reverse_proxy_empty_account_behavior,
\treverse_proxy_fixed_account_header, allocation_policy,
\tpassive_circuit_breaker_disabled, updated_at_ns,
\tlatency_top_n, latency_max_ms, latency_p95_max_ms, min_alive_streak,
\tmin_probe_success_rate, max_last_probe_age_ns, min_node_age_ns,
\tegress_ip_version, egress_country_filters_json, asn_filters_json,
\tip_type_filters_json, egress_ip_filters_json`

// scanPlatform scans one row of platformColumns.
func scanPlatform(scanner interface{ Scan(dest ...any) error }) (*model.Platform, error) {
	var p model.Platform
	var regexFiltersJSON, regionFiltersJSON string
	var countryFiltersJSON, asnFiltersJSON, ipTypeFiltersJSON, ipFiltersJSON string
	var passiveCircuitBreakerDisabled int
	if err := scanner.Scan(
		&p.ID, &p.Name, &p.StickyTTLNs, &regexFiltersJSON, &regionFiltersJSON,
		&p.ReverseProxyMissAction, &p.ReverseProxyEmptyAccountBehavior,
		&p.ReverseProxyFixedAccountHeader, &p.AllocationPolicy, &passiveCircuitBreakerDisabled, &p.UpdatedAtNs,
		&p.LatencyTopN, &p.LatencyMaxMs, &p.LatencyP95MaxMs, &p.MinAliveStreak,
		&p.MinProbeSuccessRate, &p.MaxLastProbeAgeNs, &p.MinNodeAgeNs,
		&p.EgressIPVersion, &countryFiltersJSON, &asnFiltersJSON,
		&ipTypeFiltersJSON, &ipFiltersJSON,
	); err != nil {
		return nil, err
	}
	p.PassiveCircuitBreakerDisabled = passiveCircuitBreakerDisabled != 0
	var err error
	if p.RegexFilters, err = decodeStringSliceJSON(regexFiltersJSON); err != nil {
		return nil, fmt.Errorf("decode platform %s regex_filters_json: %w", p.ID, err)
	}
	if p.RegionFilters, err = decodeStringSliceJSON(regionFiltersJSON); err != nil {
		return nil, fmt.Errorf("decode platform %s region_filters_json: %w", p.ID, err)
	}
	if p.EgressCountryFilters, err = decodeStringSliceJSON(countryFiltersJSON); err != nil {
		return nil, fmt.Errorf("decode platform %s egress_country_filters_json: %w", p.ID, err)
	}
	if p.ASNFilters, err = decodeStringSliceJSON(asnFiltersJSON); err != nil {
		return nil, fmt.Errorf("decode platform %s asn_filters_json: %w", p.ID, err)
	}
	if p.IPTypeFilters, err = decodeStringSliceJSON(ipTypeFiltersJSON); err != nil {
		return nil, fmt.Errorf("decode platform %s ip_type_filters_json: %w", p.ID, err)
	}
	if p.EgressIPFilters, err = decodeStringSliceJSON(ipFiltersJSON); err != nil {
		return nil, fmt.Errorf("decode platform %s egress_ip_filters_json: %w", p.ID, err)
	}
	return &p, nil
}

// UpsertPlatform inserts or updates a platform by ID.
// If the name collides with a different platform's name, ErrConflict is returned.
func (r *StateRepo) UpsertPlatform(p model.Platform) error {
	p.Name = platform.NormalizePlatformName(p.Name)
	if err := platform.ValidatePlatformName(p.Name); err != nil {
		return fmt.Errorf("platform name: %w", err)
	}

	if _, err := platform.CompileRegexFilters(p.RegexFilters); err != nil {
		return err
	}
	if err := platform.ValidateRegionFilters(p.RegionFilters); err != nil {
		return err
	}
	if err := platform.Validate(platform.PolicyFromModel(p)); err != nil {
		return err
	}
	missAction := platform.NormalizeReverseProxyMissAction(p.ReverseProxyMissAction)
	if missAction == "" {
		return fmt.Errorf("reverse_proxy_miss_action: invalid value %q", p.ReverseProxyMissAction)
	}
	p.ReverseProxyMissAction = string(missAction)
	if !platform.AllocationPolicy(p.AllocationPolicy).IsValid() {
		return fmt.Errorf("allocation_policy: invalid value %q", p.AllocationPolicy)
	}
	behavior := platform.ReverseProxyEmptyAccountBehavior(strings.TrimSpace(p.ReverseProxyEmptyAccountBehavior))
	if behavior == "" {
		behavior = platform.ReverseProxyEmptyAccountBehaviorRandom
	}
	if !behavior.IsValid() {
		return fmt.Errorf("reverse_proxy_empty_account_behavior: invalid value %q", p.ReverseProxyEmptyAccountBehavior)
	}
	p.ReverseProxyEmptyAccountBehavior = string(behavior)
	normalizedFixedHeaders, fixedHeaders, err := platform.NormalizeFixedAccountHeaders(p.ReverseProxyFixedAccountHeader)
	if err != nil {
		return fmt.Errorf("reverse_proxy_fixed_account_header: %w", err)
	}
	p.ReverseProxyFixedAccountHeader = normalizedFixedHeaders
	if behavior == platform.ReverseProxyEmptyAccountBehaviorFixedHeader && len(fixedHeaders) == 0 {
		return fmt.Errorf(
			"reverse_proxy_fixed_account_header: required when reverse_proxy_empty_account_behavior is %s",
			platform.ReverseProxyEmptyAccountBehaviorFixedHeader,
		)
	}

	regexFiltersJSON, err := encodeStringSliceJSON(p.RegexFilters)
	if err != nil {
		return fmt.Errorf("encode platform %s regex_filters: %w", p.ID, err)
	}
	regionFiltersJSON, err := encodeStringSliceJSON(p.RegionFilters)
	if err != nil {
		return fmt.Errorf("encode platform %s region_filters: %w", p.ID, err)
	}
	countryFiltersJSON, err := encodeStringSliceJSON(p.EgressCountryFilters)
	if err != nil {
		return fmt.Errorf("encode platform %s egress_country_filters: %w", p.ID, err)
	}
	asnFiltersJSON, err := encodeStringSliceJSON(p.ASNFilters)
	if err != nil {
		return fmt.Errorf("encode platform %s asn_filters: %w", p.ID, err)
	}
	ipTypeFiltersJSON, err := encodeStringSliceJSON(p.IPTypeFilters)
	if err != nil {
		return fmt.Errorf("encode platform %s ip_type_filters: %w", p.ID, err)
	}
	ipFiltersJSON, err := encodeStringSliceJSON(p.EgressIPFilters)
	if err != nil {
		return fmt.Errorf("encode platform %s egress_ip_filters: %w", p.ID, err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	_, err = r.db.Exec(`
		INSERT INTO platforms (id, name, sticky_ttl_ns, regex_filters_json, region_filters_json,
		                       reverse_proxy_miss_action, reverse_proxy_empty_account_behavior,
		                       reverse_proxy_fixed_account_header, allocation_policy,
		                       passive_circuit_breaker_disabled, updated_at_ns,
		                       latency_top_n, latency_max_ms, latency_p95_max_ms, min_alive_streak,
		                       min_probe_success_rate, max_last_probe_age_ns, min_node_age_ns,
		                       egress_ip_version, egress_country_filters_json, asn_filters_json,
		                       ip_type_filters_json, egress_ip_filters_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name                     = excluded.name,
			sticky_ttl_ns            = excluded.sticky_ttl_ns,
			regex_filters_json       = excluded.regex_filters_json,
			region_filters_json      = excluded.region_filters_json,
			reverse_proxy_miss_action = excluded.reverse_proxy_miss_action,
			reverse_proxy_empty_account_behavior = excluded.reverse_proxy_empty_account_behavior,
			reverse_proxy_fixed_account_header   = excluded.reverse_proxy_fixed_account_header,
			allocation_policy        = excluded.allocation_policy,
			passive_circuit_breaker_disabled = excluded.passive_circuit_breaker_disabled,
			updated_at_ns            = excluded.updated_at_ns,
			latency_top_n            = excluded.latency_top_n,
			latency_max_ms           = excluded.latency_max_ms,
			latency_p95_max_ms       = excluded.latency_p95_max_ms,
			min_alive_streak         = excluded.min_alive_streak,
			min_probe_success_rate   = excluded.min_probe_success_rate,
			max_last_probe_age_ns    = excluded.max_last_probe_age_ns,
			min_node_age_ns          = excluded.min_node_age_ns,
			egress_ip_version        = excluded.egress_ip_version,
			egress_country_filters_json = excluded.egress_country_filters_json,
			asn_filters_json         = excluded.asn_filters_json,
			ip_type_filters_json     = excluded.ip_type_filters_json,
			egress_ip_filters_json   = excluded.egress_ip_filters_json
	`, p.ID, p.Name, p.StickyTTLNs, regexFiltersJSON, regionFiltersJSON,
		p.ReverseProxyMissAction, p.ReverseProxyEmptyAccountBehavior, p.ReverseProxyFixedAccountHeader,
		p.AllocationPolicy, p.PassiveCircuitBreakerDisabled, p.UpdatedAtNs,
		p.LatencyTopN, p.LatencyMaxMs, p.LatencyP95MaxMs, p.MinAliveStreak,
		p.MinProbeSuccessRate, p.MaxLastProbeAgeNs, p.MinNodeAgeNs,
		p.EgressIPVersion, countryFiltersJSON, asnFiltersJSON,
		ipTypeFiltersJSON, ipFiltersJSON)
	if err != nil {
		if isSQLiteUniqueConstraint(err) {
			return fmt.Errorf("%w: platform name already exists", ErrConflict)
		}
		return err
	}
	return err
}

func isSQLiteUniqueConstraint(err error) bool {
	var sqlErr *sqlite.Error
	if !errors.As(err, &sqlErr) {
		return false
	}
	switch sqlErr.Code() {
	case sqlite3.SQLITE_CONSTRAINT_UNIQUE:
		return true
	}
	return false
}

// DeletePlatform removes a platform by ID.
func (r *StateRepo) DeletePlatform(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	result, err := r.db.Exec("DELETE FROM platforms WHERE id = ?", id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// GetPlatformName returns platform name by ID without decoding filter columns.
func (r *StateRepo) GetPlatformName(id string) (string, error) {
	row := r.db.QueryRow(`SELECT name FROM platforms WHERE id = ?`, id)
	var name string
	if err := row.Scan(&name); err != nil {
		if err == sql.ErrNoRows {
			return "", ErrNotFound
		}
		return "", err
	}
	return name, nil
}

// GetPlatform returns one platform by ID.
func (r *StateRepo) GetPlatform(id string) (*model.Platform, error) {
	row := r.db.QueryRow(`SELECT `+platformColumns+` FROM platforms WHERE id = ?`, id)
	p, err := scanPlatform(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return p, nil
}

// ListPlatforms returns all platforms.
func (r *StateRepo) ListPlatforms() ([]model.Platform, error) {
	rows, err := r.db.Query(`SELECT ` + platformColumns + ` FROM platforms`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []model.Platform
	for rows.Next() {
		p, err := scanPlatform(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *p)
	}
	return result, rows.Err()
}

// --- platform egress-IP ledger ---

// ListPlatformEgressIPs returns the ledger rows for one platform.
func (r *StateRepo) ListPlatformEgressIPs(platformID string) ([]model.PlatformEgressIP, error) {
	rows, err := r.db.Query(`
		SELECT platform_id, ip, state, failure_count, success_streak, last_result,
		       last_probe_at_ns, last_success_at_ns, first_seen_ns, last_seen_ns
		FROM platform_egress_ips WHERE platform_id = ? ORDER BY last_seen_ns DESC`, platformID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.PlatformEgressIP
	for rows.Next() {
		var e model.PlatformEgressIP
		if err := rows.Scan(&e.PlatformID, &e.IP, &e.State, &e.FailureCount, &e.SuccessStreak,
			&e.LastResult, &e.LastProbeAtNs, &e.LastSuccessAtNs, &e.FirstSeenNs, &e.LastSeenNs); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// UpsertPlatformEgressIP inserts or updates one ledger row.
func (r *StateRepo) UpsertPlatformEgressIP(e model.PlatformEgressIP) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, err := r.db.Exec(`
		INSERT INTO platform_egress_ips (platform_id, ip, state, failure_count, success_streak,
		                                 last_result, last_probe_at_ns, last_success_at_ns,
		                                 first_seen_ns, last_seen_ns)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(platform_id, ip) DO UPDATE SET
			state             = excluded.state,
			failure_count     = excluded.failure_count,
			success_streak    = excluded.success_streak,
			last_result       = excluded.last_result,
			last_probe_at_ns  = excluded.last_probe_at_ns,
			last_success_at_ns = excluded.last_success_at_ns,
			last_seen_ns      = excluded.last_seen_ns
	`, e.PlatformID, e.IP, e.State, e.FailureCount, e.SuccessStreak,
		e.LastResult, e.LastProbeAtNs, e.LastSuccessAtNs, e.FirstSeenNs, e.LastSeenNs)
	return err
}

// GCPlatformEgressIPs removes ledger rows not seen since olderThanNs.
func (r *StateRepo) GCPlatformEgressIPs(olderThanNs int64) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	res, err := r.db.Exec(`DELETE FROM platform_egress_ips WHERE last_seen_ns > 0 AND last_seen_ns < ?`, olderThanNs)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

'''

s = s[:start] + block + s[end:]
p.write_text(s, encoding='utf-8')
print('OK repo_state platforms block replaced')

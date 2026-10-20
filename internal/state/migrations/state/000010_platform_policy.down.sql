DROP INDEX IF EXISTS idx_platform_egress_ips_last_seen;
DROP TABLE IF EXISTS platform_egress_ips;

ALTER TABLE platforms DROP COLUMN latency_top_n;
ALTER TABLE platforms DROP COLUMN latency_max_ms;
ALTER TABLE platforms DROP COLUMN latency_p95_max_ms;
ALTER TABLE platforms DROP COLUMN min_alive_streak;
ALTER TABLE platforms DROP COLUMN min_probe_success_rate;
ALTER TABLE platforms DROP COLUMN max_last_probe_age_ns;
ALTER TABLE platforms DROP COLUMN min_node_age_ns;
ALTER TABLE platforms DROP COLUMN egress_ip_version;
ALTER TABLE platforms DROP COLUMN egress_country_filters_json;
ALTER TABLE platforms DROP COLUMN asn_filters_json;
ALTER TABLE platforms DROP COLUMN ip_type_filters_json;
ALTER TABLE platforms DROP COLUMN egress_ip_filters_json;

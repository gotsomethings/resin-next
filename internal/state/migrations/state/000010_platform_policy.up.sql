-- Platform policy: 11-dimension filter surface + per-platform egress IP ledger.
-- Additive only (ADD COLUMN / CREATE TABLE) to keep rollback simple.

ALTER TABLE platforms ADD COLUMN latency_top_n INTEGER NOT NULL DEFAULT 0;
ALTER TABLE platforms ADD COLUMN latency_max_ms INTEGER NOT NULL DEFAULT 0;
ALTER TABLE platforms ADD COLUMN latency_p95_max_ms INTEGER NOT NULL DEFAULT 0;
ALTER TABLE platforms ADD COLUMN min_alive_streak INTEGER NOT NULL DEFAULT 0;
ALTER TABLE platforms ADD COLUMN min_probe_success_rate REAL NOT NULL DEFAULT 0;
ALTER TABLE platforms ADD COLUMN max_last_probe_age_ns INTEGER NOT NULL DEFAULT 0;
ALTER TABLE platforms ADD COLUMN min_node_age_ns INTEGER NOT NULL DEFAULT 0;
ALTER TABLE platforms ADD COLUMN egress_ip_version TEXT NOT NULL DEFAULT '';
ALTER TABLE platforms ADD COLUMN egress_country_filters_json TEXT NOT NULL DEFAULT '[]';
ALTER TABLE platforms ADD COLUMN asn_filters_json TEXT NOT NULL DEFAULT '[]';
ALTER TABLE platforms ADD COLUMN ip_type_filters_json TEXT NOT NULL DEFAULT '[]';
ALTER TABLE platforms ADD COLUMN egress_ip_filters_json TEXT NOT NULL DEFAULT '[]';

CREATE TABLE IF NOT EXISTS platform_egress_ips (
	platform_id      TEXT    NOT NULL,
	ip               TEXT    NOT NULL,
	state            TEXT    NOT NULL DEFAULT 'unknown',
	failure_count    INTEGER NOT NULL DEFAULT 0,
	success_streak   INTEGER NOT NULL DEFAULT 0,
	last_result      TEXT    NOT NULL DEFAULT '',
	last_probe_at_ns INTEGER NOT NULL DEFAULT 0,
	last_success_at_ns INTEGER NOT NULL DEFAULT 0,
	first_seen_ns    INTEGER NOT NULL DEFAULT 0,
	last_seen_ns     INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (platform_id, ip)
);

CREATE INDEX IF NOT EXISTS idx_platform_egress_ips_last_seen
	ON platform_egress_ips (platform_id, last_seen_ns);

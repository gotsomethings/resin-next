-- P2: per-platform probe configuration + verdict/blacklist state machine.

ALTER TABLE platforms ADD COLUMN probe_enabled INTEGER NOT NULL DEFAULT 0;
ALTER TABLE platforms ADD COLUMN probe_url TEXT NOT NULL DEFAULT '';
ALTER TABLE platforms ADD COLUMN probe_method TEXT NOT NULL DEFAULT 'POST';
ALTER TABLE platforms ADD COLUMN probe_headers TEXT NOT NULL DEFAULT '';
ALTER TABLE platforms ADD COLUMN probe_body TEXT NOT NULL DEFAULT '';
ALTER TABLE platforms ADD COLUMN probe_pass_status_json TEXT NOT NULL DEFAULT '[200]';
ALTER TABLE platforms ADD COLUMN probe_follow_redirects INTEGER NOT NULL DEFAULT 1;
ALTER TABLE platforms ADD COLUMN probe_interval_ns INTEGER NOT NULL DEFAULT 1800000000000;
ALTER TABLE platforms ADD COLUMN probe_scope TEXT NOT NULL DEFAULT 'untested,failed,blacklisted';
ALTER TABLE platforms ADD COLUMN verdict_enabled INTEGER NOT NULL DEFAULT 0;
ALTER TABLE platforms ADD COLUMN blacklist_threshold INTEGER NOT NULL DEFAULT 3;
ALTER TABLE platforms ADD COLUMN rehab_interval_ns INTEGER NOT NULL DEFAULT 21600000000000;
ALTER TABLE platforms ADD COLUMN rehab_success_threshold INTEGER NOT NULL DEFAULT 3;
ALTER TABLE platforms ADD COLUMN blacklist_empty_policy TEXT NOT NULL DEFAULT 'strict';
ALTER TABLE platforms ADD COLUMN blacklist_grace_k INTEGER NOT NULL DEFAULT 0;

ALTER TABLE platform_egress_ips ADD COLUMN last_fail_at_ns INTEGER NOT NULL DEFAULT 0;
ALTER TABLE platform_egress_ips ADD COLUMN blacklisted_at_ns INTEGER NOT NULL DEFAULT 0;
ALTER TABLE platform_egress_ips ADD COLUMN last_node_hash TEXT NOT NULL DEFAULT '';

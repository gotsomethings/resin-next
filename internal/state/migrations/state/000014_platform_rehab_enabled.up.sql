-- P5: per-platform 「黑名单复用」(rehab) switch.
-- When 0, blacklisted egress IPs are never re-probed.
ALTER TABLE platforms ADD COLUMN rehab_enabled INTEGER NOT NULL DEFAULT 0;

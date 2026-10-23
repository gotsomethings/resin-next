-- Manual blacklist + last probe status code on the egress-IP ledger.
ALTER TABLE platform_egress_ips ADD COLUMN last_status_code INTEGER NOT NULL DEFAULT 0;
ALTER TABLE platform_egress_ips ADD COLUMN manual_blacklist INTEGER NOT NULL DEFAULT 0;

-- P4: per-platform failover (connect stage only).
ALTER TABLE platforms ADD COLUMN failover_enabled INTEGER NOT NULL DEFAULT 0;
ALTER TABLE platforms ADD COLUMN failover_max_switches INTEGER NOT NULL DEFAULT 3;
ALTER TABLE platforms ADD COLUMN failover_deadline_ns INTEGER NOT NULL DEFAULT 15000000000;
ALTER TABLE platforms ADD COLUMN failover_stage TEXT NOT NULL DEFAULT 'connect';

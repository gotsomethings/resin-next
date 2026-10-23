-- P5: per-platform probe success-rate window (ns). 0 = fixed 24h view.
ALTER TABLE platforms ADD COLUMN probe_rate_window_ns INTEGER NOT NULL DEFAULT 0;

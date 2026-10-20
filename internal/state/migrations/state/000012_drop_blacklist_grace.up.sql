-- Remove the grace fallback: blacklist is strict-only.
ALTER TABLE platforms DROP COLUMN blacklist_empty_policy;
ALTER TABLE platforms DROP COLUMN blacklist_grace_k;

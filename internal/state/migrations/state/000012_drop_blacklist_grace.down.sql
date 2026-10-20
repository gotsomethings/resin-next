-- Restore the removed columns (schema compatibility only; unused).
ALTER TABLE platforms ADD COLUMN blacklist_empty_policy TEXT NOT NULL DEFAULT 'strict';
ALTER TABLE platforms ADD COLUMN blacklist_grace_k INTEGER NOT NULL DEFAULT 0;

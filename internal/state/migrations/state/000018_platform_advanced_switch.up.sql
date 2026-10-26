-- Advanced configuration master switch (advanced tab). Existing platforms keep
-- their behaviour: advanced filtering stays on.
ALTER TABLE platforms ADD COLUMN advanced_enabled INTEGER NOT NULL DEFAULT 1;

CREATE UNIQUE INDEX IF NOT EXISTS signing_keys_single_active_idx
    ON signing_keys (active)
    WHERE active;

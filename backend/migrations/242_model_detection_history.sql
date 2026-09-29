CREATE TABLE IF NOT EXISTS model_detection_history (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    source VARCHAR(16) NOT NULL CHECK (source IN ('manual', 'scheduled', 'legacy')),
    snapshot JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_model_detection_history_account_id
    ON model_detection_history (account_id, id DESC);

-- Preserve the only result available from versions that stored just a snapshot.
INSERT INTO model_detection_history (account_id, source, snapshot)
SELECT a.id, 'legacy', a.extra -> 'model_detection_snapshot'
FROM accounts a
WHERE a.deleted_at IS NULL
  AND jsonb_typeof(a.extra -> 'model_detection_snapshot') = 'object'
  AND NOT EXISTS (SELECT 1 FROM model_detection_history h WHERE h.account_id = a.id);

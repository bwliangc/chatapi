-- Keep short-lived samples of published dynamic base multipliers so the
-- group-rate dashboard can render an accurate 24-hour change curve.
CREATE TABLE IF NOT EXISTS group_dynamic_rate_history (
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    recorded_at TIMESTAMPTZ NOT NULL,
    rate_multiplier DECIMAL(10,4) NOT NULL,
    PRIMARY KEY (group_id, recorded_at)
);

CREATE INDEX IF NOT EXISTS idx_group_dynamic_rate_history_recorded_at
    ON group_dynamic_rate_history(recorded_at);

-- Give newly upgraded installations an honest starting point. Earlier values
-- cannot be reconstructed from the single current value stored on groups.
INSERT INTO group_dynamic_rate_history (group_id, recorded_at, rate_multiplier)
SELECT id, NOW(), rate_multiplier
FROM groups
WHERE deleted_at IS NULL AND dynamic_rate ->> 'enabled' = 'true'
ON CONFLICT (group_id, recorded_at) DO NOTHING;

COMMENT ON TABLE group_dynamic_rate_history IS 'Published dynamic group base multiplier samples; retained for the rate dashboard';

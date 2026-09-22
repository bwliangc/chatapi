-- The published base multiplier remains in rate_multiplier, so billing,
-- profit admission and user-visible pricing consume the same value.
ALTER TABLE groups ADD COLUMN IF NOT EXISTS dynamic_rate JSONB NOT NULL DEFAULT '{}';
ALTER TABLE groups ADD COLUMN IF NOT EXISTS dynamic_rate_updated_at TIMESTAMPTZ;

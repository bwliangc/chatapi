-- Each campaign owns an immutable, pre-generated reward schedule.
CREATE TABLE lucky_second_campaigns (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(120) NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL,
    timezone VARCHAR(80) NOT NULL,
    total_amount NUMERIC(20,8) NOT NULL CHECK (total_amount > 0),
    reward_count INTEGER NOT NULL CHECK (reward_count BETWEEN 1 AND 10000),
    cancelled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (ends_at > starts_at)
);
CREATE TABLE lucky_second_slots (
    id BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES lucky_second_campaigns(id),
    second_at TIMESTAMPTZ NOT NULL,
    amount NUMERIC(20,8) NOT NULL CHECK (amount > 0),
    state VARCHAR(16) NOT NULL DEFAULT 'waiting' CHECK (state IN ('waiting','awarded','expired','cancelled')),
    user_id BIGINT REFERENCES users(id),
    request_id TEXT,
    awarded_at TIMESTAMPTZ,
    cache_invalidated BOOLEAN NOT NULL DEFAULT FALSE,
    UNIQUE (campaign_id, second_at)
);
CREATE INDEX lucky_second_slots_waiting ON lucky_second_slots(second_at) WHERE state = 'waiting';
CREATE TABLE lucky_second_workers (
    id TEXT PRIMARY KEY,
    heartbeat_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE lucky_second_candidates (
    id BIGSERIAL PRIMARY KEY,
    slot_id BIGINT NOT NULL REFERENCES lucky_second_slots(id),
    attempt_id TEXT NOT NULL,
    worker_id TEXT NOT NULL REFERENCES lucky_second_workers(id),
    arrived_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    state VARCHAR(16) NOT NULL DEFAULT 'pending' CHECK (state IN ('pending','valid','invalid')),
    user_id BIGINT REFERENCES users(id),
    api_key_id BIGINT,
    request_id TEXT,
    UNIQUE (slot_id, attempt_id)
);
CREATE INDEX lucky_second_candidates_attempt ON lucky_second_candidates(attempt_id);
CREATE INDEX lucky_second_candidates_order ON lucky_second_candidates(slot_id, id);
CREATE INDEX lucky_second_candidates_pending ON lucky_second_candidates(worker_id) WHERE state = 'pending';

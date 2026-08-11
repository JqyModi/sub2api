CREATE TABLE IF NOT EXISTS growth_events (
    id BIGSERIAL PRIMARY KEY,
    event_type VARCHAR(64) NOT NULL,
    campaign_id VARCHAR(100) NOT NULL DEFAULT '',
    platform VARCHAR(32) NOT NULL DEFAULT '',
    app_version VARCHAR(32) NOT NULL DEFAULT '',
    session_hash VARCHAR(64) NOT NULL DEFAULT '',
    user_id BIGINT NULL,
    plan_id BIGINT NULL,
    payment_provider VARCHAR(32) NOT NULL DEFAULT '',
    result VARCHAR(32) NOT NULL DEFAULT 'success',
    error_code VARCHAR(100) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS growth_events_event_created_idx
    ON growth_events (event_type, created_at);
CREATE INDEX IF NOT EXISTS growth_events_campaign_created_idx
    ON growth_events (campaign_id, created_at);
CREATE INDEX IF NOT EXISTS growth_events_session_hash_idx
    ON growth_events (session_hash);
CREATE INDEX IF NOT EXISTS growth_events_user_created_idx
    ON growth_events (user_id, created_at);

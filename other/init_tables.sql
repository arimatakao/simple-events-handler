CREATE TABLE IF NOT EXISTS events (
    id SERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    action TEXT NOT NULL,
    metadata_page TEXT,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS user_event_counts (
    user_id BIGINT NOT NULL,
    period_start TIMESTAMPTZ NOT NULL,
    period_end TIMESTAMPTZ NOT NULL,
    event_count BIGINT NOT NULL,
    PRIMARY KEY (user_id, period_start)
);

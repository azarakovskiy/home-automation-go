CREATE TABLE IF NOT EXISTS announcer_state (
    key TEXT PRIMARY KEY,
    announced_at BIGINT NOT NULL
);

INSERT INTO announcer_state (key, announced_at)
VALUES ('morning_summary', 0)
ON CONFLICT (key) DO NOTHING;

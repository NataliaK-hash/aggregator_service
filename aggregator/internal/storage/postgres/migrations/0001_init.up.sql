CREATE TABLE IF NOT EXISTS packet_max (
    id TEXT PRIMARY KEY,
    timestamp TIMESTAMPTZ NOT NULL,
    max_value INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_packet_max_timestamp ON packet_max (timestamp);

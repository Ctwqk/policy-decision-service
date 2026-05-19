CREATE SCHEMA IF NOT EXISTS pds;

CREATE TABLE IF NOT EXISTS pds.decisions (
  id              UUID PRIMARY KEY,
  ts              TIMESTAMPTZ NOT NULL DEFAULT now(),
  actor_id        TEXT NOT NULL,
  action_type     TEXT NOT NULL,
  platform        TEXT,
  verdict         TEXT NOT NULL,
  score           NUMERIC(4,3),
  reasons         JSONB NOT NULL,
  evaluated_rules TEXT[] NOT NULL,
  request         JSONB NOT NULL,
  latency_us      INT NOT NULL,
  rules_version   TEXT NOT NULL,
  client          TEXT
);

CREATE INDEX IF NOT EXISTS decisions_actor_ts_idx ON pds.decisions (actor_id, ts DESC);
CREATE INDEX IF NOT EXISTS decisions_verdict_ts_idx ON pds.decisions (verdict, ts DESC);
CREATE INDEX IF NOT EXISTS decisions_action_type_ts_idx ON pds.decisions (action_type, ts DESC);

CREATE TABLE IF NOT EXISTS pds.actor_profile_cache (
  actor_id    TEXT PRIMARY KEY,
  age_days    INT,
  flags_24h   INT,
  blocks_7d   INT,
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);


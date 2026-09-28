CREATE TABLE IF NOT EXISTS teams (
    id   TEXT PRIMARY KEY,
    name TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS team_aliases (
    source  TEXT NOT NULL,
    alias   TEXT NOT NULL,
    team_id TEXT NOT NULL REFERENCES teams (id),
    PRIMARY KEY (source, alias)
);

CREATE TABLE IF NOT EXISTS events (
    id           INTEGER PRIMARY KEY,
    league       TEXT    NOT NULL,
    home_team_id TEXT    NOT NULL REFERENCES teams (id),
    away_team_id TEXT    NOT NULL REFERENCES teams (id),
    starts_at    INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS events_by_matchup ON events (home_team_id, away_team_id, starts_at);

CREATE TABLE IF NOT EXISTS event_source_refs (
    source            TEXT    NOT NULL,
    provider_event_id TEXT    NOT NULL,
    event_id          INTEGER NOT NULL REFERENCES events (id),
    PRIMARY KEY (source, provider_event_id)
);

CREATE TABLE IF NOT EXISTS observations (
    id                INTEGER PRIMARY KEY,
    content_key       TEXT    NOT NULL UNIQUE,
    source            TEXT    NOT NULL,
    book              TEXT    NOT NULL,
    event_id          INTEGER NOT NULL REFERENCES events (id),
    market            TEXT    NOT NULL,
    side              TEXT    NOT NULL,
    decimal_price     REAL    NOT NULL,
    raw_price         TEXT    NOT NULL,
    raw_format        TEXT    NOT NULL,
    observed_at       INTEGER NOT NULL,
    received_at       INTEGER NOT NULL,
    last_confirmed_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS observations_by_price_key
    ON observations (event_id, book, market, side, source, observed_at);

CREATE TABLE IF NOT EXISTS resolved_prices (
    event_id          INTEGER NOT NULL REFERENCES events (id),
    book              TEXT    NOT NULL,
    market            TEXT    NOT NULL,
    side              TEXT    NOT NULL,
    decimal_price     REAL    NOT NULL,
    source            TEXT    NOT NULL,
    observed_at       INTEGER NOT NULL,
    last_confirmed_at INTEGER NOT NULL,
    resolved_at       INTEGER NOT NULL,
    PRIMARY KEY (event_id, book, market, side)
);

CREATE TABLE IF NOT EXISTS slice_health (
    source               TEXT    NOT NULL,
    slice                TEXT    NOT NULL,
    last_attempt_at      INTEGER NOT NULL,
    last_success_at      INTEGER,
    last_error           TEXT,
    last_error_at        INTEGER,
    consecutive_failures INTEGER NOT NULL DEFAULT 0,
    next_attempt_at      INTEGER NOT NULL,
    PRIMARY KEY (source, slice)
);

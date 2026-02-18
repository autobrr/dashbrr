CREATE TABLE IF NOT EXISTS users
(
    id            SERIAL PRIMARY KEY,
    username      TEXT UNIQUE NOT NULL,
    email         TEXT UNIQUE NOT NULL,
    password_hash TEXT        NOT NULL,
    created_at    TIMESTAMP   NOT NULL,
    updated_at    TIMESTAMP   NOT NULL
);

CREATE TABLE IF NOT EXISTS service_configurations
(
    id           SERIAL PRIMARY KEY,
    instance_id  TEXT UNIQUE NOT NULL,
    display_name TEXT        NOT NULL,
    url          TEXT,
    api_key      TEXT,
    access_url   TEXT
);

CREATE TABLE IF NOT EXISTS ui_collapse_preferences
(
    id             SERIAL PRIMARY KEY,
    user_id        BIGINT  NOT NULL,
    preference_key TEXT    NOT NULL,
    is_collapsed   BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at     TIMESTAMP NOT NULL,
    UNIQUE(user_id, preference_key)
);

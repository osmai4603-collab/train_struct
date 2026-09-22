CREATE TABLE IF NOT EXISTS app_users (
    id            BIGSERIAL PRIMARY KEY,
    username      TEXT        NOT NULL UNIQUE,
    display_name  TEXT        NOT NULL DEFAULT '',
    password_hash TEXT        NOT NULL,
    is_active     BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_app_users_username ON app_users (username);
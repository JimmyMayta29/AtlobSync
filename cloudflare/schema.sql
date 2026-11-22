-- ============================================================================
-- ATLOB Sync · SUNAT v2.0 — Esquema Cloudflare D1 (SQLite Edge)
-- ============================================================================

CREATE TABLE IF NOT EXISTS licenses (
    id               TEXT PRIMARY KEY,
    license_key      TEXT UNIQUE NOT NULL,
    plan_type        TEXT NOT NULL DEFAULT 'demo'
                     CHECK (plan_type IN ('demo', 'mensual', 'anual', 'permanente')),
    plan_label       TEXT NOT NULL DEFAULT 'Demo 7 días',
    client_name      TEXT NOT NULL DEFAULT '',
    client_contact   TEXT NOT NULL DEFAULT '',
    client_email     TEXT NOT NULL DEFAULT '',
    client_phone     TEXT NOT NULL DEFAULT '',
    status           TEXT NOT NULL DEFAULT 'active'
                     CHECK (status IN ('active', 'expired', 'revoked', 'suspended')),
    duration_days    INTEGER,
    max_companies    INTEGER NOT NULL DEFAULT 3 CHECK (max_companies >= 0),
    machine_hash     TEXT,
    machine_name     TEXT NOT NULL DEFAULT '',
    activated_at     TEXT,
    expires_at       TEXT,
    last_seen_at     TEXT,
    app_version      TEXT NOT NULL DEFAULT '',
    companies_count  INTEGER NOT NULL DEFAULT 0,
    activation_count INTEGER NOT NULL DEFAULT 0,
    heartbeat_count  INTEGER NOT NULL DEFAULT 0,
    notes            TEXT NOT NULL DEFAULT '',
    created_at       TEXT NOT NULL,
    updated_at       TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS license_events (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    license_id   TEXT,
    license_key  TEXT NOT NULL,
    event_type   TEXT NOT NULL,
    machine_hash TEXT NOT NULL DEFAULT '',
    machine_name TEXT NOT NULL DEFAULT '',
    app_version  TEXT NOT NULL DEFAULT '',
    detail       TEXT NOT NULL DEFAULT '',
    created_at   TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_licenses_status ON licenses(status);
CREATE INDEX IF NOT EXISTS idx_licenses_last_seen ON licenses(last_seen_at);
CREATE INDEX IF NOT EXISTS idx_licenses_client_email ON licenses(client_email);
CREATE INDEX IF NOT EXISTS idx_licenses_machine_hash ON licenses(machine_hash);
CREATE INDEX IF NOT EXISTS idx_license_events_license_id ON license_events(license_id, created_at);

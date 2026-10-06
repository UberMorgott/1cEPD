CREATE TABLE sessions (
    token_hash TEXT PRIMARY KEY,
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL,
    ip         TEXT NOT NULL DEFAULT '',
    user_agent TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);

CREATE TABLE edo_snapshots (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    period     TEXT NOT NULL,
    taken_at   INTEGER NOT NULL,
    row_count  INTEGER NOT NULL,
    csv_sha256 TEXT NOT NULL,
    -- baseline: первый снимок периода, для него сравнение не делается,
    -- иначе импорт породил бы лавину фиктивных «пропаж».
    is_baseline INTEGER NOT NULL DEFAULT 0,
    raw_csv    BLOB NOT NULL
);
CREATE INDEX idx_edo_snapshots_period ON edo_snapshots(period, taken_at);
-- Один и тот же отчёт, полученный повторно, не создаёт новый снимок.
CREATE UNIQUE INDEX idx_edo_snapshots_content ON edo_snapshots(period, csv_sha256);

CREATE TABLE edo_rows (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    snapshot_id     INTEGER NOT NULL REFERENCES edo_snapshots(id) ON DELETE CASCADE,
    partner_code    TEXT NOT NULL DEFAULT '',
    owner           TEXT NOT NULL DEFAULT '',
    login           TEXT NOT NULL DEFAULT '',
    edo_id          TEXT NOT NULL DEFAULT '',
    client_name     TEXT NOT NULL DEFAULT '',
    inn             TEXT NOT NULL DEFAULT '',
    kpp             TEXT NOT NULL DEFAULT '',
    its_tariffs     TEXT NOT NULL DEFAULT '',
    limit_docs      INTEGER,
    invoices_out    INTEGER NOT NULL DEFAULT 0,
    non_invoices_out INTEGER NOT NULL DEFAULT 0,
    packets         INTEGER NOT NULL DEFAULT 0,
    packets_by_owner INTEGER NOT NULL DEFAULT 0,
    discount        INTEGER NOT NULL DEFAULT 0,
    packets_billable INTEGER NOT NULL DEFAULT 0,
    tariff_amount   INTEGER NOT NULL DEFAULT 0,
    client_amount   INTEGER NOT NULL DEFAULT 0,
    partner_amount  INTEGER NOT NULL DEFAULT 0,
    extra           TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_edo_rows_snapshot ON edo_rows(snapshot_id);
CREATE INDEX idx_edo_rows_edo_id ON edo_rows(edo_id);
-- ИНН недостаточен как ключ организации: филиалы делят ИНН и различаются КПП,
-- у ИП КПП пуст. Ищем всегда по паре.
CREATE INDEX idx_edo_rows_org ON edo_rows(inn, kpp);
CREATE INDEX idx_edo_rows_login ON edo_rows(login);

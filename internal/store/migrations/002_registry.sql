-- Накопительный реестр идентификаторов ЭДО.
-- Реестра связей в партнёрском API нет, поэтому сервис собирает его сам
-- из отчётов и хранит как последнее известное состояние.
CREATE TABLE edo_identifiers (
    edo_id          TEXT PRIMARY KEY,
    inn             TEXT NOT NULL DEFAULT '',
    kpp             TEXT NOT NULL DEFAULT '',
    client_name     TEXT NOT NULL DEFAULT '',
    login           TEXT NOT NULL DEFAULT '',
    owner_raw       TEXT NOT NULL DEFAULT '',
    owner_code      TEXT NOT NULL DEFAULT '',
    its_tariffs     TEXT NOT NULL DEFAULT '',
    limit_docs      INTEGER,
    packets         INTEGER NOT NULL DEFAULT 0,
    first_seen_at   INTEGER NOT NULL,
    last_seen_at    INTEGER NOT NULL,
    last_period     TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_edo_identifiers_org ON edo_identifiers(inn, kpp);
CREATE INDEX idx_edo_identifiers_login ON edo_identifiers(login);

-- Найденные аномалии. state_fingerprint — отпечаток состояния, на основании
-- которого возникла находка: подтверждение «это законно» гасит только его,
-- изменившееся состояние поднимет сигнал заново.
CREATE TABLE identifier_events (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    edo_id            TEXT NOT NULL,
    kind              TEXT NOT NULL,
    confidence        TEXT NOT NULL,
    state_fingerprint TEXT NOT NULL,
    inn               TEXT NOT NULL DEFAULT '',
    kpp               TEXT NOT NULL DEFAULT '',
    client_name       TEXT NOT NULL DEFAULT '',
    login             TEXT NOT NULL DEFAULT '',
    details           TEXT NOT NULL DEFAULT '',
    detected_at       INTEGER NOT NULL
);
CREATE UNIQUE INDEX idx_identifier_events_unique
    ON identifier_events(edo_id, kind, state_fingerprint);
CREATE INDEX idx_identifier_events_detected ON identifier_events(detected_at);

-- Пометки «это законно», привязанные к отпечатку состояния.
CREATE TABLE identifier_acks (
    edo_id            TEXT NOT NULL,
    state_fingerprint TEXT NOT NULL,
    reason            TEXT NOT NULL DEFAULT '',
    author            TEXT NOT NULL DEFAULT '',
    acked_at          INTEGER NOT NULL,
    PRIMARY KEY (edo_id, state_fingerprint)
);

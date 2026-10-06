-- Черновики заявок. Статуса «отправлена» здесь нет: отправку сервис научился
-- делать позже, и факт письма отмечается не статусом, а колонкой sent_at,
-- которую добавляет 004_settings.sql. Статусы остались прежними: draft, exported.
CREATE TABLE its_requests (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    title        TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'draft',
    schema_ver   TEXT NOT NULL DEFAULT '3.09',
    revision     INTEGER NOT NULL DEFAULT 1,
    payload_json TEXT NOT NULL,
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL,
    exported_at  INTEGER
);
CREATE INDEX idx_its_requests_updated ON its_requests(updated_at DESC);

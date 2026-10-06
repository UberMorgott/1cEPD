-- Попытки догрузить биллинг ЭДО за прошлые месяцы (история за 12 месяцев).
-- Отчёт тратит часовой лимит 1С, поэтому догрузка идёт по одному месяцу за
-- интервал; по времени последней попытки она продолжается после перезапуска.
-- status: done — снимок сохранён, none — у 1С биллинга за месяц нет
-- (BILLING_DOES_NOT_EXIST или пустой отчёт), failed — сбой, повтор позже.
CREATE TABLE billing_backfill (
    period       TEXT PRIMARY KEY,
    status       TEXT NOT NULL,
    failures     INTEGER NOT NULL DEFAULT 0,
    attempted_at INTEGER NOT NULL,
    error        TEXT NOT NULL DEFAULT ''
);

-- Расход ЭПД за последние 12 закрытых месяцев из отчёта трафика ЭДО
-- (docs/API.md §8.2) — основа подсказки выгодного тарифа ЭПД. Отчёт тратит
-- часовой лимит, поэтому строится редко, а ответ хранится здесь целиком:
-- новый прогон заменяет строки.
CREATE TABLE epd_usage (
    position    INTEGER PRIMARY KEY,
    edo_id      TEXT NOT NULL DEFAULT '',
    inn         TEXT NOT NULL DEFAULT '',
    kpp         TEXT NOT NULL DEFAULT '',
    subscriber  TEXT NOT NULL DEFAULT '',
    client_name TEXT NOT NULL DEFAULT '',
    epd_in      INTEGER NOT NULL DEFAULT 0,
    epd_out     INTEGER NOT NULL DEFAULT 0
);

-- Один прогон: за какой период строки и когда получены.
CREATE TABLE epd_usage_run (
    id          INTEGER PRIMARY KEY CHECK (id = 1),
    period_from TEXT NOT NULL,
    period_to   TEXT NOT NULL,
    fetched_at  INTEGER NOT NULL
);

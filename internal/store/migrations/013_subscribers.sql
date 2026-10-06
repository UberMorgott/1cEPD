-- Вся база абонентов партнёра из /rest/public/subscriber (docs/API.md §7).
-- Резервная копия (спека §5, subscribers_snapshot): абонент, которого 1С
-- перестала отдавать, не удаляется — last_seen_at остаётся от последней выгрузки,
-- в которой он был. Списки — JSON-массивы, как у its_checks.contracts.
CREATE TABLE subscribers (
    code          TEXT PRIMARY KEY,
    name          TEXT NOT NULL DEFAULT '',
    subjects      TEXT NOT NULL DEFAULT '[]',
    reg_numbers   TEXT NOT NULL DEFAULT '[]',
    organizations TEXT NOT NULL DEFAULT '[]',
    first_seen_at INTEGER NOT NULL,
    last_seen_at  INTEGER NOT NULL
);

-- Последняя успешная выгрузка базы: одна строка.
CREATE TABLE subscribers_run (
    id         INTEGER PRIMARY KEY CHECK (id = 1),
    fetched_at INTEGER NOT NULL,
    total      INTEGER NOT NULL
);

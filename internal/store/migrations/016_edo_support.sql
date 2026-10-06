-- Список «Сопровождение клиентов 1С-ЭДО» партнёрского портала — наши клиенты.
-- API его не отдаёт: партнёр загружает выгрузку файлом, и она заменяет таблицу
-- целиком (список на портале — полная правда). Даты — ГГГГ-ММ-ДД, пусто — не
-- заполнена; imported_at — Unix-время загрузки, у всех строк одно.
CREATE TABLE edo_support (
    edo_id          TEXT PRIMARY KEY,
    inn             TEXT NOT NULL DEFAULT '',
    kpp             TEXT NOT NULL DEFAULT '',
    org_name        TEXT NOT NULL DEFAULT '',
    subscriber_code TEXT NOT NULL DEFAULT '',
    subscriber_name TEXT NOT NULL DEFAULT '',
    operator        TEXT NOT NULL DEFAULT '',
    support_from    TEXT NOT NULL DEFAULT '',
    support_to      TEXT NOT NULL DEFAULT '',
    id_status       TEXT NOT NULL DEFAULT '',
    login           TEXT NOT NULL DEFAULT '',
    email           TEXT NOT NULL DEFAULT '',
    link_created    TEXT NOT NULL DEFAULT '',
    link_author     TEXT NOT NULL DEFAULT '',
    id_registered   TEXT NOT NULL DEFAULT '',
    warnings        TEXT NOT NULL DEFAULT '',
    comment         TEXT NOT NULL DEFAULT '',
    imported_at     INTEGER NOT NULL
);

CREATE INDEX edo_support_inn ON edo_support (inn);

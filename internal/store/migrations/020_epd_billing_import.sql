-- Выгрузка «Детализация биллинга» ЭПД, загруженная руками (docs/API.md §8.4):
-- в партнёрском API её нет. Хранится последняя загрузка целиком — новая
-- заменяет прежнюю.
CREATE TABLE epd_billing_import (
    id          INTEGER PRIMARY KEY CHECK (id = 1),
    imported_at INTEGER NOT NULL,
    file_name   TEXT NOT NULL DEFAULT '',
    total       INTEGER NOT NULL
);

CREATE TABLE epd_billing_import_rows (
    line          INTEGER PRIMARY KEY,
    owner         TEXT NOT NULL DEFAULT '',
    owner_code    TEXT NOT NULL DEFAULT '',
    owner_contact TEXT NOT NULL DEFAULT '',
    login         TEXT NOT NULL DEFAULT '',
    edo_id        TEXT NOT NULL DEFAULT '',
    client_name   TEXT NOT NULL DEFAULT '',
    inn           TEXT NOT NULL DEFAULT '',
    kpp           TEXT NOT NULL DEFAULT '',
    its_tariffs   TEXT NOT NULL DEFAULT '',
    epd_docs      INTEGER NOT NULL DEFAULT 0
);

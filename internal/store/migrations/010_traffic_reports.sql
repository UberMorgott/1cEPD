-- Отчётов трафика ЭДО теперь два: расход ЭПД за 12 закрытых месяцев (year) и
-- текущий месяц с начала до момента построения (month) — для прогноза
-- перерасхода лимита. Строки и прогоны обоих живут в общих таблицах с ключом
-- report; прежние данные расхода ЭПД переносятся как year.
CREATE TABLE traffic_rows (
    report      TEXT NOT NULL,
    position    INTEGER NOT NULL,
    edo_id      TEXT NOT NULL DEFAULT '',
    inn         TEXT NOT NULL DEFAULT '',
    kpp         TEXT NOT NULL DEFAULT '',
    subscriber  TEXT NOT NULL DEFAULT '',
    client_name TEXT NOT NULL DEFAULT '',
    epd_in      INTEGER NOT NULL DEFAULT 0,
    epd_out     INTEGER NOT NULL DEFAULT 0,
    sf_out      INTEGER NOT NULL DEFAULT 0,
    non_sf_out  INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (report, position)
);

INSERT INTO traffic_rows (report, position, edo_id, inn, kpp, subscriber, client_name, epd_in, epd_out)
SELECT 'year', position, edo_id, inn, kpp, subscriber, client_name, epd_in, epd_out FROM epd_usage;

CREATE TABLE traffic_runs (
    report      TEXT PRIMARY KEY,
    period_from TEXT NOT NULL,
    period_to   TEXT NOT NULL,
    fetched_at  INTEGER NOT NULL
);

INSERT INTO traffic_runs (report, period_from, period_to, fetched_at)
SELECT 'year', period_from, period_to, fetched_at FROM epd_usage_run;

DROP TABLE epd_usage;
DROP TABLE epd_usage_run;

-- Последняя проверка договоров 1С:ИТС по коду абонента-владельца
-- (checkItsBySubscriberCode, docs/API.md §5). Держим ответ целиком: сроки
-- договоров нужны напоминаниям о продлении, а ходить в 1С на каждый показ
-- экрана незачем — проверка раз в сутки.
CREATE TABLE its_checks (
    subscriber_code TEXT PRIMARY KEY,
    -- code — числовой код статуса: по нему сравнивают, строка статуса пляшет.
    code            INTEGER NOT NULL,
    status          TEXT NOT NULL DEFAULT '',
    description     TEXT NOT NULL DEFAULT '',
    -- contracts — JSON-массив договоров из itsContractInfo.
    contracts       TEXT NOT NULL DEFAULT '[]',
    checked_at      INTEGER NOT NULL
);

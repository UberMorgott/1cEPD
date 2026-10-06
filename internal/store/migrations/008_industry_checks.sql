-- Последняя проверка отраслевого сопровождения по коду абонента-владельца
-- (checkIndustryBySubscriberCode, docs/API.md §6): нужен ли клиенту ИТС
-- Отраслевой и оформлен ли он. Обновляется вместе с проверкой договоров ИТС.
CREATE TABLE industry_checks (
    subscriber_code TEXT PRIMARY KEY,
    code            INTEGER NOT NULL,
    status          TEXT NOT NULL DEFAULT '',
    description     TEXT NOT NULL DEFAULT '',
    -- programs — JSON-массив programInfoList с оформленными подписками.
    programs        TEXT NOT NULL DEFAULT '[]',
    checked_at      INTEGER NOT NULL
);

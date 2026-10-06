-- Последний отчёт по опциям сервисов (option/billing-report, docs/API.md §8.3)
-- каждого вида: 1С-Отчетность, 1С:Подпись и т. д. Отчёт — JSON, держим его
-- разобранным в entries (JSON-массив абонентов с тарифами и опциями), как
-- its_checks.contracts. Каждый вид строится отдельным отчётом и тратит часовой
-- лимит 1С, поэтому fetched_at у каждого свой.
CREATE TABLE option_reports (
    report_type TEXT PRIMARY KEY,
    -- state: ok — отчёт построен; none — 1С ответила BILLING_DOES_NOT_EXIST.
    state       TEXT NOT NULL,
    entries     TEXT NOT NULL DEFAULT '[]',
    fetched_at  INTEGER NOT NULL
);

-- Когда отчёт биллинга за период строился в последний раз. Совпавший с прошлым
-- отчёт новый снимок не создаёт, поэтому по edo_snapshots.taken_at не понять,
-- свежи ли данные: без этой отметки каждый перезапуск строил бы отчёт заново
-- и тратил часовой лимит задач 1С.
CREATE TABLE billing_fetches (
    period     TEXT PRIMARY KEY,
    fetched_at INTEGER NOT NULL
);

INSERT INTO billing_fetches (period, fetched_at)
SELECT period, MAX(taken_at) FROM edo_snapshots GROUP BY period;

-- Остальные колонки отчёта трафика ЭДО: оператор, даты связи и регистрации
-- идентификатора, сопровождение, входящие СФ и прочие документы.
ALTER TABLE traffic_rows ADD COLUMN operator TEXT NOT NULL DEFAULT '';
ALTER TABLE traffic_rows ADD COLUMN support_from TEXT NOT NULL DEFAULT '';
ALTER TABLE traffic_rows ADD COLUMN support_to TEXT NOT NULL DEFAULT '';
ALTER TABLE traffic_rows ADD COLUMN link_created TEXT NOT NULL DEFAULT '';
ALTER TABLE traffic_rows ADD COLUMN id_registered TEXT NOT NULL DEFAULT '';
ALTER TABLE traffic_rows ADD COLUMN sf_in INTEGER NOT NULL DEFAULT 0;
ALTER TABLE traffic_rows ADD COLUMN non_sf_in INTEGER NOT NULL DEFAULT 0;

-- Годовой отчёт сохранён без этих колонок. Сбрасываем период прогона: на
-- ближайшем суточном прогоне отчёт построится заново — один отчёт из лимита 1С.
UPDATE traffic_runs SET period_from = '' WHERE report = 'year';

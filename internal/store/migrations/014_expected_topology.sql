-- Дата пересмотра пометки «это законно» (спека §4.2): раз в полгода скрытые
-- находки показываются для перепроверки. Прежним пометкам — полгода от даты.
ALTER TABLE identifier_acks ADD COLUMN review_at INTEGER NOT NULL DEFAULT 0;
UPDATE identifier_acks SET review_at = acked_at + 182 * 24 * 60 * 60;

-- Реестр ожидаемой топологии (спека §4.2, §5): известные людям законные связи
-- «организация → идентификатор → назначение». Законный дубль структурно не
-- отличить от поломки, поэтому его отмечают вручную с причиной и автором.
CREATE TABLE expected_topology (
    inn        TEXT NOT NULL,
    kpp        TEXT NOT NULL DEFAULT '',
    edo_id     TEXT NOT NULL,
    purpose    TEXT NOT NULL DEFAULT '',
    author     TEXT NOT NULL DEFAULT '',
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (inn, kpp, edo_id)
);

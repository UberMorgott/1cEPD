-- Программы клиента по Личному кабинету 1С — то же, что страница портала
-- «Проверка условий сопровождения»: регномер, программа, выполнены ли условия.
-- Держим последний ответ проверки на клиента (ИНН/КПП): подбор клиента в форме
-- заявки показывает его без похода в 1С, а новая проверка заменяет строки целиком.
CREATE TABLE client_programs (
    inn        TEXT NOT NULL,
    kpp        TEXT NOT NULL DEFAULT '',
    -- position — порядок строк, как их вернула 1С.
    position   INTEGER NOT NULL,
    login      TEXT NOT NULL DEFAULT '',
    reg_number TEXT NOT NULL DEFAULT '',
    program    TEXT NOT NULL DEFAULT '',
    has_access INTEGER NOT NULL,
    -- missing — JSON-массив названий недостающих условий сопровождения.
    missing    TEXT NOT NULL DEFAULT '[]',
    checked_at INTEGER NOT NULL,
    PRIMARY KEY (inn, kpp, position)
);

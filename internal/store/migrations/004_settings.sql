-- Настройки отправки почты. Строка ровно одна: настройки общие для сервиса,
-- CHECK(id = 1) не даёт завести вторую и гадать, какая из них действует.
-- Пароль лежит зашифрованным (AES-256-GCM, nonce||ciphertext); ключ — файл
-- secret.key рядом с базой, поэтому дамп базы сам по себе пароль не выдаёт.
CREATE TABLE settings (
    id                INTEGER PRIMARY KEY CHECK (id = 1),
    smtp_host         TEXT NOT NULL DEFAULT '',
    smtp_port         INTEGER NOT NULL DEFAULT 465,
    smtp_login        TEXT NOT NULL DEFAULT '',
    smtp_password_enc BLOB,
    smtp_from         TEXT NOT NULL DEFAULT '',
    -- mail_to — JSON-массив адресов: получателей несколько, а отдельная таблица
    -- ради списка из двух строк не окупается.
    mail_to           TEXT NOT NULL DEFAULT '[]',
    updated_at        INTEGER NOT NULL
);
-- mail_to по умолчанию — робот 1С, единственный штатный получатель формы (docs/spec:282).
INSERT INTO settings (id, smtp_host, smtp_port, mail_to, updated_at)
VALUES (1, 'smtp.yandex.ru', 465, '["itsrobot@1c.ru"]', 0);

-- Отметка об отправке письма живёт рядом с отметкой о выгрузке файла.
ALTER TABLE its_requests ADD COLUMN sent_at INTEGER;

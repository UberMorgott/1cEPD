-- Отправитель заявки ИТС: шапка формы заявки, одна на партнёра. Пароль заявки
-- лежит зашифрованным тем же ключом, что и пароль SMTP.
ALTER TABLE settings ADD COLUMN its_partner_code TEXT NOT NULL DEFAULT '';
ALTER TABLE settings ADD COLUMN its_responsible TEXT NOT NULL DEFAULT '';
ALTER TABLE settings ADD COLUMN its_email TEXT NOT NULL DEFAULT '';
ALTER TABLE settings ADD COLUMN its_password_enc BLOB;

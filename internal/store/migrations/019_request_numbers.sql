-- Порядковый номер заявки: «Заявка N от ДД.ММ.ГГГГ». Сохранённые раньше
-- нумеруются по порядку создания, самая старая — первая; новые продолжают
-- после наибольшего номера.
ALTER TABLE its_requests ADD COLUMN number INTEGER NOT NULL DEFAULT 0;

UPDATE its_requests SET number = (
    SELECT COUNT(*) FROM its_requests AS earlier
    WHERE earlier.created_at < its_requests.created_at
       OR (earlier.created_at = its_requests.created_at AND earlier.id <= its_requests.id)
);

-- Название по старому образцу «Заявка <код партнёра> от <дата>» получает номер,
-- чтобы название и номер в списке совпадали. Своё название не трогаем.
UPDATE its_requests
SET title = 'Заявка ' || number || ' от ' || substr(title, -10)
WHERE title LIKE 'Заявка % от __.__.____';

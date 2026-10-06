-- Самообновление с GitHub: проверять ли новые версии и ставить ли их сами.
-- По умолчанию включено и то и другое — как у agent-link.
ALTER TABLE settings ADD COLUMN update_check INTEGER NOT NULL DEFAULT 1;
ALTER TABLE settings ADD COLUMN update_install INTEGER NOT NULL DEFAULT 1;

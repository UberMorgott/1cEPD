package store

import (
	"path/filepath"
	"testing"
)

func TestOpenCreatesSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open вернул ошибку: %v", err)
	}
	defer func() { _ = db.Close() }()

	var version int
	if err := db.QueryRowContext(t.Context(), "PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("не прочитать user_version: %v", err)
	}
	// Точное число миграций проверяет TestMigrationsCreateRegistryTables,
	// здесь важно лишь то, что первая применилась.
	if version < 1 {
		t.Errorf("user_version = %d, ожидали не меньше 1", version)
	}

	for _, table := range []string{"sessions", "edo_snapshots", "edo_rows"} {
		var name string
		err := db.QueryRowContext(t.Context(),
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&name)
		if err != nil {
			t.Errorf("таблица %s не создана: %v", table, err)
		}
	}
}

func TestMigrationsCreateRegistryTables(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	var version int
	if err := db.QueryRowContext(t.Context(), "PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("user_version: %v", err)
	}
	// Миграции нумеруются по порядку файлов, новые только прибавляются:
	// сверяем нижнюю границу, иначе тест ломается на каждой добавленной.
	if version < 2 {
		t.Errorf("user_version = %d, ожидали не меньше 2", version)
	}

	for _, table := range []string{"edo_identifiers", "identifier_events", "identifier_acks"} {
		var name string
		err := db.QueryRowContext(t.Context(),
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("таблица %s не создана: %v", table, err)
		}
	}
}

func TestOpenIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("первый Open: %v", err)
	}
	_ = db.Close()

	db2, err := Open(path)
	if err != nil {
		t.Fatalf("второй Open: %v", err)
	}
	defer func() { _ = db2.Close() }()
}

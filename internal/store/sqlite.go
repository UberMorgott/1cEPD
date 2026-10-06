// Package store отвечает за хранение данных в SQLite.
package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Open открывает базу, создаёт каталог при необходимости и применяет миграции.
func Open(path string) (*sql.DB, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("store: не создать каталог %s: %w", dir, err)
		}
	}

	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: не открыть базу: %w", err)
	}

	// Один писатель: SQLite не любит параллельную запись, а нагрузка здесь мизерная.
	db.SetMaxOpenConns(1)

	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	// Файл базы создаёт драйвер, и права у него общедоступные на чтение,
	// а внутри — сессии и настройки почты. Сужаем после миграций: до них
	// файла может ещё не быть. На Windows вызов ничего не меняет.
	if err := os.Chmod(path, 0o600); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: не ограничить права на %s: %w", path, err)
	}
	return db, nil
}

// migrate применяет миграции, которых ещё нет, ориентируясь на PRAGMA user_version.
func migrate(db *sql.DB) error {
	// Миграции идут при старте, до появления запросов: своего контекста здесь нет.
	ctx := context.Background()

	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("store: не прочитать миграции: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)

	var current int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&current); err != nil {
		return fmt.Errorf("store: не прочитать user_version: %w", err)
	}

	for i, name := range names {
		version := i + 1
		if version <= current {
			continue
		}
		body, err := migrationFiles.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("store: не прочитать %s: %w", name, err)
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("store: не начать транзакцию для %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, string(body)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("store: миграция %s не применилась: %w", name, err)
		}
		// PRAGMA не принимает параметры, поэтому число подставляется в текст запроса.
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", version)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("store: не обновить user_version после %s: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("store: не зафиксировать %s: %w", name, err)
		}
	}
	return nil
}

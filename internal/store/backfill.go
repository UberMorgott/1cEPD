package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Статусы догрузки биллинга за прошлый месяц.
const (
	BackfillDone   = "done"
	BackfillNone   = "none"
	BackfillFailed = "failed"
	// BackfillLimited — 1С упёрлась в лимит отчётов: повтор позже, в счёт сбоев не идёт.
	BackfillLimited = "limited"
)

// BackfillAttempt — последняя попытка догрузить месяц.
type BackfillAttempt struct {
	Period      string
	Status      string
	Failures    int
	AttemptedAt time.Time
	Error       string
}

// Backfill хранит попытки догрузки истории биллинга.
type Backfill struct {
	db *sql.DB
}

// NewBackfill создаёт хранилище поверх открытой базы.
func NewBackfill(db *sql.DB) *Backfill {
	return &Backfill{db: db}
}

// Record сохраняет итог попытки. Сбой копит счётчик, успех и «биллинга нет»
// его не трогают: счётчик нужен, чтобы бросить месяц, который не строится.
func (s *Backfill) Record(ctx context.Context, period, status, errText string, at time.Time) error {
	failed := 0
	if status == BackfillFailed {
		failed = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO billing_backfill (period, status, failures, attempted_at, error)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(period) DO UPDATE SET status = excluded.status,
			failures = billing_backfill.failures + excluded.failures,
			attempted_at = excluded.attempted_at, error = excluded.error`,
		period, status, failed, at.UTC().Unix(), errText)
	if err != nil {
		return fmt.Errorf("store: не записать попытку догрузки %s: %w", period, err)
	}
	return nil
}

// Attempts возвращает попытки, ключ — месяц ГГГГ-ММ.
func (s *Backfill) Attempts(ctx context.Context) (map[string]BackfillAttempt, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT period, status, failures, attempted_at, error FROM billing_backfill`)
	if err != nil {
		return nil, fmt.Errorf("store: не прочитать попытки догрузки: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string]BackfillAttempt)
	for rows.Next() {
		var a BackfillAttempt
		var at int64
		if err := rows.Scan(&a.Period, &a.Status, &a.Failures, &at, &a.Error); err != nil {
			return nil, fmt.Errorf("store: не разобрать попытку догрузки: %w", err)
		}
		a.AttemptedAt = time.Unix(at, 0).UTC()
		result[a.Period] = a
	}
	return result, rows.Err()
}

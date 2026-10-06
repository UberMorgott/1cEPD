package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"partnerops/internal/partner"
)

// Отчёты трафика ЭДО, которые хранит сервис.
const (
	// TrafficYear — расход за 12 закрытых месяцев, основа подсказки тарифа ЭПД.
	TrafficYear = "year"
	// TrafficMonth — текущий месяц с первого числа, основа прогноза перерасхода.
	TrafficMonth = "month"
)

// EPDUsageRun — метаданные последнего отчёта трафика.
type EPDUsageRun struct {
	// PeriodFrom и PeriodTo — месяцы отчёта, ГГГГ-ММ, включительно.
	PeriodFrom string
	PeriodTo   string
	FetchedAt  time.Time
}

// EPDUsage хранит последний построенный отчёт трафика одного вида.
type EPDUsage struct {
	db     *sql.DB
	report string
}

// NewEPDUsage создаёт хранилище расхода ЭПД за 12 месяцев.
func NewEPDUsage(db *sql.DB) *EPDUsage {
	return &EPDUsage{db: db, report: TrafficYear}
}

// NewMonthTraffic создаёт хранилище трафика текущего месяца.
func NewMonthTraffic(db *sql.DB) *EPDUsage {
	return &EPDUsage{db: db, report: TrafficMonth}
}

// Replace заменяет строки и метаданные результатом нового отчёта.
func (s *EPDUsage) Replace(ctx context.Context, run EPDUsageRun, rows []partner.EDOTrafficRow) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: не начать транзакцию отчёта трафика: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM traffic_rows WHERE report = ?`, s.report); err != nil {
		return fmt.Errorf("store: не очистить отчёт трафика: %w", err)
	}
	for position, r := range rows {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO traffic_rows (report, position, edo_id, inn, kpp, subscriber, client_name,
				epd_in, epd_out, sf_out, non_sf_out, sf_in, non_sf_in, operator,
				support_from, support_to, link_created, id_registered)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			s.report, position, r.EDOID, r.INN, r.KPP, r.Subscriber, r.ClientName,
			r.EPDIn, r.EPDOut, r.InvoicesOut, r.NonInvoicesOut, r.InvoicesIn, r.NonInvoicesIn, r.Operator,
			r.SupportFrom, r.SupportTo, r.LinkCreated, r.IDRegistered); err != nil {
			return fmt.Errorf("store: не сохранить строку отчёта трафика: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO traffic_runs (report, period_from, period_to, fetched_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(report) DO UPDATE SET period_from = excluded.period_from,
			period_to = excluded.period_to, fetched_at = excluded.fetched_at`,
		s.report, run.PeriodFrom, run.PeriodTo, run.FetchedAt.UTC().Unix()); err != nil {
		return fmt.Errorf("store: не сохранить прогон отчёта трафика: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: не зафиксировать отчёт трафика: %w", err)
	}
	return nil
}

// Run возвращает метаданные последнего отчёта; ok false — отчёта ещё не было.
func (s *EPDUsage) Run(ctx context.Context) (EPDUsageRun, bool, error) {
	var run EPDUsageRun
	var fetched int64
	err := s.db.QueryRowContext(ctx,
		`SELECT period_from, period_to, fetched_at FROM traffic_runs WHERE report = ?`, s.report,
	).Scan(&run.PeriodFrom, &run.PeriodTo, &fetched)
	if errors.Is(err, sql.ErrNoRows) {
		return EPDUsageRun{}, false, nil
	}
	if err != nil {
		return EPDUsageRun{}, false, fmt.Errorf("store: не прочитать прогон отчёта трафика: %w", err)
	}
	run.FetchedAt = time.Unix(fetched, 0).UTC()
	return run, true, nil
}

// Rows возвращает строки последнего отчёта в исходном порядке.
func (s *EPDUsage) Rows(ctx context.Context) ([]partner.EDOTrafficRow, error) {
	cursor, err := s.db.QueryContext(ctx, `
		SELECT edo_id, inn, kpp, subscriber, client_name, epd_in, epd_out, sf_out, non_sf_out,
		       sf_in, non_sf_in, operator, support_from, support_to, link_created, id_registered
		FROM traffic_rows WHERE report = ? ORDER BY position`, s.report)
	if err != nil {
		return nil, fmt.Errorf("store: не прочитать отчёт трафика: %w", err)
	}
	defer func() { _ = cursor.Close() }()

	var rows []partner.EDOTrafficRow
	for cursor.Next() {
		var r partner.EDOTrafficRow
		if err := cursor.Scan(&r.EDOID, &r.INN, &r.KPP, &r.Subscriber, &r.ClientName,
			&r.EPDIn, &r.EPDOut, &r.InvoicesOut, &r.NonInvoicesOut, &r.InvoicesIn, &r.NonInvoicesIn,
			&r.Operator, &r.SupportFrom, &r.SupportTo, &r.LinkCreated, &r.IDRegistered); err != nil {
			return nil, fmt.Errorf("store: не разобрать отчёт трафика: %w", err)
		}
		rows = append(rows, r)
	}
	return rows, cursor.Err()
}

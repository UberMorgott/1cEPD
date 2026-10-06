package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"partnerops/internal/money"
	"partnerops/internal/partner"
)

// ErrNoSnapshot означает, что снапшотов за период ещё нет.
var ErrNoSnapshot = errors.New("store: снапшот не найден")

// Snapshot — метаданные сохранённого отчёта.
type Snapshot struct {
	ID         int64
	Period     string
	TakenAt    time.Time
	RowCount   int64
	IsBaseline bool
}

// Snapshots хранит снапшоты отчёта биллинга ЭДО.
type Snapshots struct {
	db *sql.DB
}

// NewSnapshots создаёт хранилище поверх открытой базы.
func NewSnapshots(db *sql.DB) *Snapshots {
	return &Snapshots{db: db}
}

// SaveResult описывает, что произошло при сохранении.
type SaveResult struct {
	ID         int64
	IsBaseline bool // первый снимок периода: сравнивать не с чем
	Duplicate  bool // такой же отчёт уже сохранён, ничего не записано
}

// Save сохраняет отчёт целиком: сырой CSV и разобранные строки. Всё в одной транзакции,
// чтобы не появилось снапшота без строк.
//
// Повторно полученный байт в байт отчёт не сохраняется: 1С отдаёт одни и те же данные
// при нескольких запусках за день, и лишние снимки породили бы пустые сравнения.
func (s *Snapshots) Save(
	ctx context.Context, period string, takenAt time.Time, raw []byte, rows []partner.EDOBillingRow,
) (SaveResult, error) {
	sum := sha256.Sum256(raw)
	digest := hex.EncodeToString(sum[:])

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return SaveResult{}, fmt.Errorf("store: не начать транзакцию: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Отчёт построен — отмечаем, даже если он совпал с прошлым.
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO billing_fetches (period, fetched_at) VALUES (?, ?)
		ON CONFLICT(period) DO UPDATE SET fetched_at = excluded.fetched_at`,
		period, takenAt.UTC().Unix()); err != nil {
		return SaveResult{}, fmt.Errorf("store: не отметить построение отчёта: %w", err)
	}

	var existingID int64
	err = tx.QueryRowContext(ctx,
		`SELECT id FROM edo_snapshots WHERE period = ? AND csv_sha256 = ?`, period, digest,
	).Scan(&existingID)
	if err == nil {
		if err := tx.Commit(); err != nil {
			return SaveResult{}, fmt.Errorf("store: не зафиксировать отметку отчёта: %w", err)
		}
		return SaveResult{ID: existingID, Duplicate: true}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return SaveResult{}, fmt.Errorf("store: не проверить дубликат снапшота: %w", err)
	}

	var priorCount int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM edo_snapshots WHERE period = ?`, period,
	).Scan(&priorCount); err != nil {
		return SaveResult{}, fmt.Errorf("store: не сосчитать снапшоты периода: %w", err)
	}
	isBaseline := priorCount == 0

	result, err := tx.ExecContext(ctx,
		`INSERT INTO edo_snapshots (period, taken_at, row_count, csv_sha256, is_baseline, raw_csv)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		period, takenAt.UTC().Unix(), len(rows), digest, isBaseline, raw,
	)
	if err != nil {
		return SaveResult{}, fmt.Errorf("store: не сохранить снапшот: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return SaveResult{}, fmt.Errorf("store: не получить идентификатор снапшота: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO edo_rows (
			snapshot_id, partner_code, owner, login, edo_id, client_name, inn, kpp,
			its_tariffs, limit_docs, invoices_out, non_invoices_out, packets,
			packets_by_owner, discount, packets_billable,
			tariff_amount, client_amount, partner_amount, extra
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return SaveResult{}, fmt.Errorf("store: не подготовить вставку строк: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, r := range rows {
		var limit any
		if r.Limit != nil {
			limit = *r.Limit
		}
		_, err := stmt.ExecContext(ctx,
			id, r.PartnerCode, r.Owner, r.Login, r.EDOID, r.ClientName, r.INN, r.KPP,
			r.ITSTariffs, limit, r.InvoicesOut, r.NonInvoicesOut, r.Packets,
			r.PacketsByOwner, r.Discount, r.PacketsBillable,
			int64(r.TariffAmount), int64(r.ClientAmount), int64(r.PartnerAmount), r.Extra,
		)
		if err != nil {
			return SaveResult{}, fmt.Errorf("store: не сохранить строку %s: %w", r.EDOID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return SaveResult{}, fmt.Errorf("store: не зафиксировать снапшот: %w", err)
	}
	return SaveResult{ID: id, IsBaseline: isBaseline}, nil
}

// Latest возвращает самый свежий снапшот за период вместе со строками.
func (s *Snapshots) Latest(ctx context.Context, period string) (Snapshot, []partner.EDOBillingRow, error) {
	var snap Snapshot
	var takenAt int64

	err := s.db.QueryRowContext(ctx,
		`SELECT id, period, taken_at, row_count, is_baseline FROM edo_snapshots
		 WHERE period = ? ORDER BY taken_at DESC, id DESC LIMIT 1`, period,
	).Scan(&snap.ID, &snap.Period, &takenAt, &snap.RowCount, &snap.IsBaseline)
	if errors.Is(err, sql.ErrNoRows) {
		return Snapshot{}, nil, ErrNoSnapshot
	}
	if err != nil {
		return Snapshot{}, nil, fmt.Errorf("store: не прочитать снапшот: %w", err)
	}
	snap.TakenAt = time.Unix(takenAt, 0).UTC()

	rows, err := s.rowsOf(ctx, snap.ID)
	if err != nil {
		return Snapshot{}, nil, err
	}
	return snap, rows, nil
}

// LastFetched возвращает, когда отчёт за период строился в последний раз,
// включая совпавшие с прошлым. Нулевое время — ни разу.
func (s *Snapshots) LastFetched(ctx context.Context, period string) (time.Time, error) {
	var at int64
	err := s.db.QueryRowContext(ctx,
		`SELECT fetched_at FROM billing_fetches WHERE period = ?`, period).Scan(&at)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("store: не прочитать время отчёта: %w", err)
	}
	return time.Unix(at, 0).UTC(), nil
}

// Periods возвращает месяцы, за которые есть хотя бы один снапшот.
func (s *Snapshots) Periods(ctx context.Context) (map[string]bool, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT period FROM edo_snapshots`)
	if err != nil {
		return nil, fmt.Errorf("store: не прочитать месяцы снапшотов: %w", err)
	}
	defer func() { _ = rows.Close() }()

	periods := make(map[string]bool)
	for rows.Next() {
		var period string
		if err := rows.Scan(&period); err != nil {
			return nil, fmt.Errorf("store: не разобрать месяц снапшота: %w", err)
		}
		periods[period] = true
	}
	return periods, rows.Err()
}

func (s *Snapshots) rowsOf(ctx context.Context, snapshotID int64) ([]partner.EDOBillingRow, error) {
	cursor, err := s.db.QueryContext(ctx, `
		SELECT partner_code, owner, login, edo_id, client_name, inn, kpp, its_tariffs,
		       limit_docs, invoices_out, non_invoices_out, packets, packets_by_owner,
		       discount, packets_billable, tariff_amount, client_amount, partner_amount, extra
		FROM edo_rows WHERE snapshot_id = ? ORDER BY id`, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("store: не прочитать строки снапшота: %w", err)
	}
	defer func() { _ = cursor.Close() }()

	var rows []partner.EDOBillingRow
	for cursor.Next() {
		var r partner.EDOBillingRow
		var limit sql.NullInt64
		var tariff, client, partnerAmount int64

		err := cursor.Scan(
			&r.PartnerCode, &r.Owner, &r.Login, &r.EDOID, &r.ClientName, &r.INN, &r.KPP,
			&r.ITSTariffs, &limit, &r.InvoicesOut, &r.NonInvoicesOut, &r.Packets,
			&r.PacketsByOwner, &r.Discount, &r.PacketsBillable,
			&tariff, &client, &partnerAmount, &r.Extra,
		)
		if err != nil {
			return nil, fmt.Errorf("store: не разобрать строку снапшота: %w", err)
		}
		if limit.Valid {
			value := limit.Int64
			r.Limit = &value
		}
		r.TariffAmount = money.Amount(tariff)
		r.ClientAmount = money.Amount(client)
		r.PartnerAmount = money.Amount(partnerAmount)
		rows = append(rows, r)
	}
	return rows, cursor.Err()
}

package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"partnerops/internal/partner"
)

// EPDImportInfo — последняя загрузка выгрузки биллинга ЭПД.
type EPDImportInfo struct {
	ImportedAt time.Time
	FileName   string
	Total      int
}

// EPDBillingImport хранит последнюю загруженную выгрузку «Детализация биллинга» ЭПД.
type EPDBillingImport struct {
	db *sql.DB
}

// NewEPDBillingImport создаёт хранилище поверх открытой базы.
func NewEPDBillingImport(db *sql.DB) *EPDBillingImport {
	return &EPDBillingImport{db: db}
}

// Replace заменяет прежнюю загрузку новой.
func (s *EPDBillingImport) Replace(ctx context.Context, fileName string, rows []partner.EPDBillingRow, at time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: не начать транзакцию выгрузки ЭПД: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM epd_billing_import_rows`); err != nil {
		return fmt.Errorf("store: не очистить выгрузку ЭПД: %w", err)
	}
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO epd_billing_import_rows (line, owner, owner_code, owner_contact, login, edo_id,
			client_name, inn, kpp, its_tariffs, epd_docs)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("store: не подготовить строку выгрузки ЭПД: %w", err)
	}
	defer func() { _ = stmt.Close() }()
	for i, r := range rows {
		if _, err := stmt.ExecContext(ctx, i+1, r.Owner, r.OwnerCode, r.OwnerContact, r.Login, r.EDOID,
			r.ClientName, r.INN, r.KPP, r.ITSTariffs, r.EPDDocs); err != nil {
			return fmt.Errorf("store: не записать строку выгрузки ЭПД: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO epd_billing_import (id, imported_at, file_name, total) VALUES (1, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET imported_at = excluded.imported_at,
			file_name = excluded.file_name, total = excluded.total`,
		at.UTC().Unix(), fileName, len(rows)); err != nil {
		return fmt.Errorf("store: не записать сведения о выгрузке ЭПД: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: не сохранить выгрузку ЭПД: %w", err)
	}
	return nil
}

// Rows отдаёт строки последней загрузки в порядке файла.
func (s *EPDBillingImport) Rows(ctx context.Context) ([]partner.EPDBillingRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT owner, owner_code, owner_contact, login, edo_id, client_name, inn, kpp, its_tariffs, epd_docs
		FROM epd_billing_import_rows ORDER BY line`)
	if err != nil {
		return nil, fmt.Errorf("store: не прочитать выгрузку ЭПД: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var list []partner.EPDBillingRow
	for rows.Next() {
		var r partner.EPDBillingRow
		if err := rows.Scan(&r.Owner, &r.OwnerCode, &r.OwnerContact, &r.Login, &r.EDOID,
			&r.ClientName, &r.INN, &r.KPP, &r.ITSTariffs, &r.EPDDocs); err != nil {
			return nil, fmt.Errorf("store: не разобрать строку выгрузки ЭПД: %w", err)
		}
		list = append(list, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: не прочитать выгрузку ЭПД: %w", err)
	}
	return list, nil
}

// Last — сведения о последней загрузке; ok false — загрузок не было.
func (s *EPDBillingImport) Last(ctx context.Context) (EPDImportInfo, bool, error) {
	var info EPDImportInfo
	var unix int64
	err := s.db.QueryRowContext(ctx, `SELECT imported_at, file_name, total FROM epd_billing_import WHERE id = 1`).
		Scan(&unix, &info.FileName, &info.Total)
	if errors.Is(err, sql.ErrNoRows) {
		return info, false, nil
	}
	if err != nil {
		return info, false, fmt.Errorf("store: не прочитать сведения о выгрузке ЭПД: %w", err)
	}
	info.ImportedAt = time.Unix(unix, 0).UTC()
	return info, true, nil
}

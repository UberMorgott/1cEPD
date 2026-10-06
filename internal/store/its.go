package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"partnerops/internal/partner"
)

// ITSCheck — сохранённая проверка договоров 1С:ИТС одного абонента.
type ITSCheck struct {
	partner.ITSCheck
	CheckedAt time.Time
}

// itsContractJSON — договор в колонке contracts. Отдельный тип, чтобы формат
// хранения не зависел от полей partner.ITSContract.
type itsContractJSON struct {
	Description     string    `json:"description"`
	Start           time.Time `json:"start"`
	End             time.Time `json:"end"`
	TypeUIN         string    `json:"typeUin"`
	TypeName        string    `json:"typeName"`
	TypeNameForUser string    `json:"typeNameForUser"`
	TypeNumber      *int      `json:"typeNumber"`
}

// ITSChecks хранит последнюю проверку договоров 1С:ИТС по каждому абоненту.
type ITSChecks struct {
	db *sql.DB
}

// NewITSChecks создаёт хранилище поверх открытой базы.
func NewITSChecks(db *sql.DB) *ITSChecks {
	return &ITSChecks{db: db}
}

// Replace заменяет все проверки результатом нового прогона: абонент, которого
// больше нет в реестре, не должен висеть в напоминаниях со старыми сроками.
func (s *ITSChecks) Replace(ctx context.Context, checks []partner.ITSCheck, at time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: не начать транзакцию проверок ИТС: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM its_checks`); err != nil {
		return fmt.Errorf("store: не очистить проверки ИТС: %w", err)
	}
	for _, check := range checks {
		contracts := make([]itsContractJSON, 0, len(check.Contracts))
		for _, c := range check.Contracts {
			contracts = append(contracts, itsContractJSON(c))
		}
		raw, err := json.Marshal(contracts)
		if err != nil {
			return fmt.Errorf("store: не сохранить договоры ИТС: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO its_checks (subscriber_code, code, status, description, contracts, checked_at)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT(subscriber_code) DO UPDATE SET
				code = excluded.code, status = excluded.status, description = excluded.description,
				contracts = excluded.contracts, checked_at = excluded.checked_at`,
			check.SubscriberCode, check.Code, check.Status, check.Description,
			string(raw), at.UTC().Unix()); err != nil {
			return fmt.Errorf("store: не сохранить проверку ИТС %s: %w", check.SubscriberCode, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: не зафиксировать проверки ИТС: %w", err)
	}
	return nil
}

// All возвращает сохранённые проверки по коду абонента.
func (s *ITSChecks) All(ctx context.Context) ([]ITSCheck, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT subscriber_code, code, status, description, contracts, checked_at
		FROM its_checks ORDER BY subscriber_code`)
	if err != nil {
		return nil, fmt.Errorf("store: не прочитать проверки ИТС: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var list []ITSCheck
	for rows.Next() {
		var check ITSCheck
		var raw string
		var checked int64
		if err := rows.Scan(&check.SubscriberCode, &check.Code, &check.Status,
			&check.Description, &raw, &checked); err != nil {
			return nil, fmt.Errorf("store: не разобрать проверку ИТС: %w", err)
		}
		var contracts []itsContractJSON
		if err := json.Unmarshal([]byte(raw), &contracts); err != nil {
			return nil, fmt.Errorf("store: не разобрать договоры ИТС: %w", err)
		}
		for _, c := range contracts {
			check.Contracts = append(check.Contracts, partner.ITSContract(c))
		}
		check.CheckedAt = time.Unix(checked, 0).UTC()
		list = append(list, check)
	}
	return list, rows.Err()
}

// LastChecked возвращает время последнего прогона; нулевое — проверок не было.
func (s *ITSChecks) LastChecked(ctx context.Context) (time.Time, error) {
	var last sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `SELECT MAX(checked_at) FROM its_checks`).Scan(&last); err != nil {
		return time.Time{}, fmt.Errorf("store: не прочитать время проверки ИТС: %w", err)
	}
	if !last.Valid {
		return time.Time{}, nil
	}
	return time.Unix(last.Int64, 0).UTC(), nil
}

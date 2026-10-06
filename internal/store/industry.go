package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"partnerops/internal/partner"
)

// IndustryCheck — сохранённая проверка отраслевого сопровождения абонента.
type IndustryCheck struct {
	partner.IndustryCheck
	CheckedAt time.Time
}

// Типы колонки programs: формат хранения не зависит от полей пакета partner.
type industrySubscriptionJSON struct {
	NomenclatureName string    `json:"nomenclatureName"`
	SerialNumber     string    `json:"serialNumber"`
	TypeDescription  string    `json:"typeDescription"`
	Begin            time.Time `json:"begin"`
	End              time.Time `json:"end"`
}

type industryProgramJSON struct {
	UIN           string                     `json:"uin"`
	Name          string                     `json:"name"`
	Subscriptions []industrySubscriptionJSON `json:"subscriptions"`
}

// IndustryChecks хранит последнюю проверку ИТС Отраслевого по каждому абоненту.
type IndustryChecks struct {
	db *sql.DB
}

// NewIndustryChecks создаёт хранилище поверх открытой базы.
func NewIndustryChecks(db *sql.DB) *IndustryChecks {
	return &IndustryChecks{db: db}
}

// Replace заменяет все проверки результатом нового прогона.
func (s *IndustryChecks) Replace(ctx context.Context, checks []partner.IndustryCheck, at time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: не начать транзакцию проверок ИТС Отраслевого: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM industry_checks`); err != nil {
		return fmt.Errorf("store: не очистить проверки ИТС Отраслевого: %w", err)
	}
	for _, check := range checks {
		programs := make([]industryProgramJSON, 0, len(check.Programs))
		for _, p := range check.Programs {
			program := industryProgramJSON{UIN: p.UIN, Name: p.Name,
				Subscriptions: make([]industrySubscriptionJSON, 0, len(p.Subscriptions))}
			for _, sub := range p.Subscriptions {
				program.Subscriptions = append(program.Subscriptions, industrySubscriptionJSON(sub))
			}
			programs = append(programs, program)
		}
		raw, err := json.Marshal(programs)
		if err != nil {
			return fmt.Errorf("store: не сохранить программы ИТС Отраслевого: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO industry_checks (subscriber_code, code, status, description, programs, checked_at)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT(subscriber_code) DO UPDATE SET
				code = excluded.code, status = excluded.status, description = excluded.description,
				programs = excluded.programs, checked_at = excluded.checked_at`,
			check.SubscriberCode, check.Code, check.Status, check.Description,
			string(raw), at.UTC().Unix()); err != nil {
			return fmt.Errorf("store: не сохранить проверку ИТС Отраслевого %s: %w", check.SubscriberCode, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: не зафиксировать проверки ИТС Отраслевого: %w", err)
	}
	return nil
}

// All возвращает сохранённые проверки по коду абонента.
func (s *IndustryChecks) All(ctx context.Context) ([]IndustryCheck, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT subscriber_code, code, status, description, programs, checked_at
		FROM industry_checks ORDER BY subscriber_code`)
	if err != nil {
		return nil, fmt.Errorf("store: не прочитать проверки ИТС Отраслевого: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var list []IndustryCheck
	for rows.Next() {
		var check IndustryCheck
		var raw string
		var checked int64
		if err := rows.Scan(&check.SubscriberCode, &check.Code, &check.Status,
			&check.Description, &raw, &checked); err != nil {
			return nil, fmt.Errorf("store: не разобрать проверку ИТС Отраслевого: %w", err)
		}
		var programs []industryProgramJSON
		if err := json.Unmarshal([]byte(raw), &programs); err != nil {
			return nil, fmt.Errorf("store: не разобрать программы ИТС Отраслевого: %w", err)
		}
		for _, p := range programs {
			program := partner.IndustryProgram{UIN: p.UIN, Name: p.Name}
			for _, sub := range p.Subscriptions {
				program.Subscriptions = append(program.Subscriptions, partner.IndustrySubscription(sub))
			}
			check.Programs = append(check.Programs, program)
		}
		check.CheckedAt = time.Unix(checked, 0).UTC()
		list = append(list, check)
	}
	return list, rows.Err()
}

// LastChecked возвращает время последнего прогона; нулевое — проверок не было.
func (s *IndustryChecks) LastChecked(ctx context.Context) (time.Time, error) {
	var last sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `SELECT MAX(checked_at) FROM industry_checks`).Scan(&last); err != nil {
		return time.Time{}, fmt.Errorf("store: не прочитать время проверки ИТС Отраслевого: %w", err)
	}
	if !last.Valid {
		return time.Time{}, nil
	}
	return time.Unix(last.Int64, 0).UTC(), nil
}

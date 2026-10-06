package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// ClientProgram — строка проверки условий сопровождения программы клиента.
type ClientProgram struct {
	INN       string
	KPP       string
	Login     string
	RegNumber string
	Program   string
	HasAccess bool
	// Missing — названия недостающих условий сопровождения.
	Missing   []string
	CheckedAt time.Time
}

// ClientPrograms хранит последнюю проверку программ по каждому клиенту.
type ClientPrograms struct {
	db *sql.DB
}

// NewClientPrograms создаёт хранилище поверх открытой базы.
func NewClientPrograms(db *sql.DB) *ClientPrograms {
	return &ClientPrograms{db: db}
}

// Replace заменяет программы клиента результатом новой проверки.
func (s *ClientPrograms) Replace(
	ctx context.Context, inn, kpp string, programs []ClientProgram, at time.Time,
) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: не начать транзакцию программ клиента: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM client_programs WHERE inn = ? AND kpp = ?`, inn, kpp); err != nil {
		return fmt.Errorf("store: не очистить программы клиента: %w", err)
	}
	for position, program := range programs {
		missing, err := json.Marshal(nonNil(program.Missing))
		if err != nil {
			return fmt.Errorf("store: не сохранить условия сопровождения: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO client_programs
			  (inn, kpp, position, login, reg_number, program, has_access, missing, checked_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			inn, kpp, position, program.Login, program.RegNumber, program.Program,
			program.HasAccess, string(missing), at.UTC().Unix()); err != nil {
			return fmt.Errorf("store: не сохранить программу клиента: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: не зафиксировать программы клиента: %w", err)
	}
	return nil
}

// All возвращает программы всех клиентов в порядке проверки.
func (s *ClientPrograms) All(ctx context.Context) ([]ClientProgram, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT inn, kpp, login, reg_number, program, has_access, missing, checked_at
		FROM client_programs ORDER BY inn, kpp, position`)
	if err != nil {
		return nil, fmt.Errorf("store: не прочитать программы клиентов: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var list []ClientProgram
	for rows.Next() {
		var program ClientProgram
		var missing string
		var checked int64
		if err := rows.Scan(&program.INN, &program.KPP, &program.Login, &program.RegNumber,
			&program.Program, &program.HasAccess, &missing, &checked); err != nil {
			return nil, fmt.Errorf("store: не разобрать программу клиента: %w", err)
		}
		if err := json.Unmarshal([]byte(missing), &program.Missing); err != nil {
			return nil, fmt.Errorf("store: не разобрать условия сопровождения: %w", err)
		}
		program.CheckedAt = time.Unix(checked, 0).UTC()
		list = append(list, program)
	}
	return list, rows.Err()
}

func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

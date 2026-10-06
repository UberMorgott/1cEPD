package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"partnerops/internal/partner"
)

// SubscriberRecord — сохранённый абонент и когда 1С его отдавала.
type SubscriberRecord struct {
	partner.Subscriber
	FirstSeenAt time.Time
	LastSeenAt  time.Time
}

// organizationJSON — организация в колонке organizations. Отдельный тип, чтобы
// формат хранения не зависел от полей partner.Organization.
type organizationJSON struct {
	Name string `json:"name"`
	INN  string `json:"inn"`
	KPP  string `json:"kpp"`
}

// Subscribers хранит базу абонентов партнёра.
type Subscribers struct {
	db *sql.DB
}

// NewSubscribers создаёт хранилище поверх открытой базы.
func NewSubscribers(db *sql.DB) *Subscribers {
	return &Subscribers{db: db}
}

// Save записывает выгрузку целиком. Известные абоненты обновляются, новые
// добавляются, пропавшие из выгрузки остаются с прежним last_seen_at.
func (s *Subscribers) Save(ctx context.Context, list []partner.Subscriber, at time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: не начать транзакцию абонентов: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO subscribers (code, name, subjects, reg_numbers, organizations, first_seen_at, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(code) DO UPDATE SET
			name = excluded.name,
			subjects = excluded.subjects,
			reg_numbers = excluded.reg_numbers,
			organizations = excluded.organizations,
			last_seen_at = excluded.last_seen_at`)
	if err != nil {
		return fmt.Errorf("store: не подготовить запись абонента: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	unix := at.UTC().Unix()
	for _, sub := range list {
		if sub.Code == "" {
			continue
		}
		orgs := make([]organizationJSON, 0, len(sub.Organizations))
		for _, org := range sub.Organizations {
			orgs = append(orgs, organizationJSON(org))
		}
		subjects, err := jsonList(sub.Subjects)
		if err != nil {
			return err
		}
		regs, err := jsonList(sub.RegNumbers)
		if err != nil {
			return err
		}
		rawOrgs, err := json.Marshal(orgs)
		if err != nil {
			return fmt.Errorf("store: не сохранить организации абонента: %w", err)
		}
		if _, err := stmt.ExecContext(ctx, sub.Code, sub.Name, subjects, regs, string(rawOrgs), unix, unix); err != nil {
			return fmt.Errorf("store: не записать абонента %s: %w", sub.Code, err)
		}
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO subscribers_run (id, fetched_at, total) VALUES (1, ?, ?)
		ON CONFLICT(id) DO UPDATE SET fetched_at = excluded.fetched_at, total = excluded.total`,
		unix, len(list)); err != nil {
		return fmt.Errorf("store: не отметить выгрузку абонентов: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: не зафиксировать абонентов: %w", err)
	}
	return nil
}

// jsonList сохраняет список строк; nil пишется пустым массивом.
func jsonList(values []string) (string, error) {
	if values == nil {
		values = []string{}
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("store: не сохранить список: %w", err)
	}
	return string(raw), nil
}

// LastFetched — время последней успешной выгрузки; нулевое — выгрузок не было.
func (s *Subscribers) LastFetched(ctx context.Context) (time.Time, error) {
	var at int64
	err := s.db.QueryRowContext(ctx, `SELECT fetched_at FROM subscribers_run WHERE id = 1`).Scan(&at)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("store: не прочитать выгрузку абонентов: %w", err)
	}
	return time.Unix(at, 0).UTC(), nil
}

// All возвращает всех когда-либо выгруженных абонентов по коду.
func (s *Subscribers) All(ctx context.Context) ([]SubscriberRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT code, name, subjects, reg_numbers, organizations, first_seen_at, last_seen_at
		FROM subscribers ORDER BY code`)
	if err != nil {
		return nil, fmt.Errorf("store: не прочитать абонентов: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []SubscriberRecord
	for rows.Next() {
		var r SubscriberRecord
		var subjects, regs, orgs string
		var first, last int64
		if err := rows.Scan(&r.Code, &r.Name, &subjects, &regs, &orgs, &first, &last); err != nil {
			return nil, fmt.Errorf("store: не разобрать абонента: %w", err)
		}
		var list []organizationJSON
		if err := errors.Join(json.Unmarshal([]byte(subjects), &r.Subjects),
			json.Unmarshal([]byte(regs), &r.RegNumbers), json.Unmarshal([]byte(orgs), &list)); err != nil {
			return nil, fmt.Errorf("store: не разобрать списки абонента %s: %w", r.Code, err)
		}
		for _, org := range list {
			r.Organizations = append(r.Organizations, partner.Organization(org))
		}
		r.FirstSeenAt = time.Unix(first, 0).UTC()
		r.LastSeenAt = time.Unix(last, 0).UTC()
		result = append(result, r)
	}
	return result, rows.Err()
}

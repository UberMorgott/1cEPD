package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrStaleRevision означает, что черновик успели изменить в другом окне.
var ErrStaleRevision = errors.New("store: черновик изменён другим пользователем")

// RequestDraft — сохранённая заявка.
type RequestDraft struct {
	ID int64
	// Number — порядковый номер заявки, назначается при создании.
	Number      int64
	Title       string
	Status      string
	SchemaVer   string
	Revision    int64
	PayloadJSON string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ExportedAt  *time.Time
	SentAt      *time.Time
}

// Requests хранит черновики заявок.
type Requests struct {
	db *sql.DB
}

// NewRequests создаёт хранилище поверх открытой базы.
func NewRequests(db *sql.DB) *Requests {
	return &Requests{db: db}
}

// Save создаёт новый черновик.
func (s *Requests) Save(ctx context.Context, draft RequestDraft, at time.Time) (int64, error) {
	unix := at.UTC().Unix()
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO its_requests (number, title, status, schema_ver, revision, payload_json, created_at, updated_at)
		VALUES ((SELECT COALESCE(MAX(number), 0) + 1 FROM its_requests), ?, ?, ?, 1, ?, ?, ?)`,
		draft.Title, draft.Status, draft.SchemaVer, draft.PayloadJSON, unix, unix)
	if err != nil {
		return 0, fmt.Errorf("store: не сохранить заявку: %w", err)
	}
	return result.LastInsertId()
}

// Update перезаписывает черновик, если его не изменили параллельно.
func (s *Requests) Update(ctx context.Context, draft RequestDraft, at time.Time) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE its_requests
		SET title = ?, status = ?, payload_json = ?, revision = revision + 1, updated_at = ?
		WHERE id = ? AND revision = ?`,
		draft.Title, draft.Status, draft.PayloadJSON, at.UTC().Unix(), draft.ID, draft.Revision)
	if err != nil {
		return fmt.Errorf("store: не обновить заявку: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: не проверить обновление заявки: %w", err)
	}
	if affected == 0 {
		return ErrStaleRevision
	}
	return nil
}

// Get читает черновик.
func (s *Requests) Get(ctx context.Context, id int64) (RequestDraft, error) {
	var draft RequestDraft
	var created, updated int64
	var exported, sent sql.NullInt64

	err := s.db.QueryRowContext(ctx, `
		SELECT id, number, title, status, schema_ver, revision, payload_json, created_at, updated_at,
		       exported_at, sent_at
		FROM its_requests WHERE id = ?`, id).
		Scan(&draft.ID, &draft.Number, &draft.Title, &draft.Status, &draft.SchemaVer, &draft.Revision,
			&draft.PayloadJSON, &created, &updated, &exported, &sent)
	if err != nil {
		return RequestDraft{}, fmt.Errorf("store: не прочитать заявку %d: %w", id, err)
	}

	draft.CreatedAt = time.Unix(created, 0).UTC()
	draft.UpdatedAt = time.Unix(updated, 0).UTC()
	draft.ExportedAt = optionalTime(exported)
	draft.SentAt = optionalTime(sent)
	return draft, nil
}

// List возвращает заявки, свежие сверху.
func (s *Requests) List(ctx context.Context) ([]RequestDraft, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, number, title, status, schema_ver, revision, payload_json, created_at, updated_at, exported_at, sent_at
		FROM its_requests ORDER BY updated_at DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("store: не прочитать список заявок: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var list []RequestDraft
	for rows.Next() {
		var draft RequestDraft
		var created, updated int64
		var exported, sent sql.NullInt64
		if err := rows.Scan(&draft.ID, &draft.Number, &draft.Title, &draft.Status, &draft.SchemaVer,
			&draft.Revision, &draft.PayloadJSON, &created, &updated, &exported, &sent); err != nil {
			return nil, fmt.Errorf("store: не разобрать заявку: %w", err)
		}
		draft.CreatedAt = time.Unix(created, 0).UTC()
		draft.UpdatedAt = time.Unix(updated, 0).UTC()
		draft.ExportedAt = optionalTime(exported)
		draft.SentAt = optionalTime(sent)
		list = append(list, draft)
	}
	return list, rows.Err()
}

// Payloads возвращает данные всех заявок, свежие сверху: из них берутся
// реквизиты клиентов для автозаполнения новой заявки.
func (s *Requests) Payloads(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT payload_json FROM its_requests ORDER BY updated_at DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("store: не прочитать данные заявок: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var list []string
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return nil, fmt.Errorf("store: не разобрать данные заявки: %w", err)
		}
		list = append(list, payload)
	}
	return list, rows.Err()
}

// optionalTime разворачивает необязательную отметку времени из базы.
func optionalTime(value sql.NullInt64) *time.Time {
	if !value.Valid {
		return nil
	}
	at := time.Unix(value.Int64, 0).UTC()
	return &at
}

// MarkExported отмечает, что по заявке выгружен файл.
func (s *Requests) MarkExported(ctx context.Context, id int64, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE its_requests SET status = 'exported', exported_at = ?, updated_at = ?
		WHERE id = ?`, at.UTC().Unix(), at.UTC().Unix(), id)
	if err != nil {
		return fmt.Errorf("store: не отметить выгрузку заявки %d: %w", id, err)
	}
	return nil
}

// MarkSent отмечает, что заявку отправили письмом.
func (s *Requests) MarkSent(ctx context.Context, id int64, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE its_requests SET sent_at = ?, updated_at = ? WHERE id = ?`,
		at.UTC().Unix(), at.UTC().Unix(), id)
	if err != nil {
		return fmt.Errorf("store: не отметить отправку заявки %d: %w", id, err)
	}
	return nil
}

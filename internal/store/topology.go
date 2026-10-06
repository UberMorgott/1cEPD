package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// TopologyEntry — известная законная связь «организация → идентификатор».
type TopologyEntry struct {
	INN       string
	KPP       string
	EDOID     string
	Purpose   string
	Author    string
	UpdatedAt time.Time
}

// Topology хранит реестр ожидаемой топологии.
type Topology struct {
	db *sql.DB
}

// NewTopology создаёт хранилище поверх открытой базы.
func NewTopology(db *sql.DB) *Topology {
	return &Topology{db: db}
}

// Put добавляет связь или обновляет её назначение.
func (s *Topology) Put(ctx context.Context, entry TopologyEntry) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO expected_topology (inn, kpp, edo_id, purpose, author, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(inn, kpp, edo_id) DO UPDATE SET
			purpose = excluded.purpose, author = excluded.author, updated_at = excluded.updated_at`,
		entry.INN, entry.KPP, entry.EDOID, entry.Purpose, entry.Author, entry.UpdatedAt.UTC().Unix())
	if err != nil {
		return fmt.Errorf("store: не сохранить связь топологии: %w", err)
	}
	return nil
}

// Remove удаляет связь: её сигналы снова станут видны.
func (s *Topology) Remove(ctx context.Context, inn, kpp, edoID string) error {
	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM expected_topology WHERE inn = ? AND kpp = ? AND edo_id = ?`, inn, kpp, edoID); err != nil {
		return fmt.Errorf("store: не удалить связь топологии: %w", err)
	}
	return nil
}

// All возвращает весь реестр, свежие сверху.
func (s *Topology) All(ctx context.Context) ([]TopologyEntry, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT inn, kpp, edo_id, purpose, author, updated_at FROM expected_topology
		ORDER BY updated_at DESC, inn, edo_id`)
	if err != nil {
		return nil, fmt.Errorf("store: не прочитать топологию: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []TopologyEntry
	for rows.Next() {
		var e TopologyEntry
		var at int64
		if err := rows.Scan(&e.INN, &e.KPP, &e.EDOID, &e.Purpose, &e.Author, &at); err != nil {
			return nil, fmt.Errorf("store: не разобрать связь топологии: %w", err)
		}
		e.UpdatedAt = time.Unix(at, 0).UTC()
		result = append(result, e)
	}
	return result, rows.Err()
}

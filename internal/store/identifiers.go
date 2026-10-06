package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// IdentifierRecord — последнее известное состояние идентификатора ЭДО.
type IdentifierRecord struct {
	EDOID       string
	INN         string
	KPP         string
	ClientName  string
	Login       string
	OwnerRaw    string
	OwnerCode   string
	ITSTariffs  string
	Limit       *int64
	Packets     int64
	FirstSeenAt time.Time
	LastSeenAt  time.Time
	LastPeriod  string
}

// AnomalyEvent — найденная аномалия.
type AnomalyEvent struct {
	ID               int64
	EDOID            string
	Kind             string
	Confidence       string
	StateFingerprint string
	INN              string
	KPP              string
	ClientName       string
	Login            string
	Details          string
	DetectedAt       time.Time
	Acknowledged     bool
}

// Identifiers хранит накопленный реестр и найденные аномалии.
type Identifiers struct {
	db *sql.DB
}

// NewIdentifiers создаёт хранилище поверх открытой базы.
func NewIdentifiers(db *sql.DB) *Identifiers {
	return &Identifiers{db: db}
}

// Upsert добавляет новые идентификаторы и обновляет известные.
// Дата первого появления сохраняется, дата последнего сдвигается.
func (s *Identifiers) Upsert(
	ctx context.Context, records []IdentifierRecord, period string, seenAt time.Time,
) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: не начать транзакцию реестра: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO edo_identifiers (
			edo_id, inn, kpp, client_name, login, owner_raw, owner_code,
			its_tariffs, limit_docs, packets, first_seen_at, last_seen_at, last_period
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(edo_id) DO UPDATE SET
			inn = excluded.inn,
			kpp = excluded.kpp,
			client_name = excluded.client_name,
			login = excluded.login,
			owner_raw = excluded.owner_raw,
			owner_code = excluded.owner_code,
			its_tariffs = excluded.its_tariffs,
			limit_docs = excluded.limit_docs,
			packets = excluded.packets,
			last_seen_at = excluded.last_seen_at,
			last_period = excluded.last_period`)
	if err != nil {
		return fmt.Errorf("store: не подготовить запись реестра: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	unix := seenAt.UTC().Unix()
	for _, r := range records {
		var limit any
		if r.Limit != nil {
			limit = *r.Limit
		}
		_, err := stmt.ExecContext(ctx,
			r.EDOID, r.INN, r.KPP, r.ClientName, r.Login, r.OwnerRaw, r.OwnerCode,
			r.ITSTariffs, limit, r.Packets, unix, unix, period)
		if err != nil {
			return fmt.Errorf("store: не записать идентификатор %s: %w", r.EDOID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: не зафиксировать реестр: %w", err)
	}
	return nil
}

// OwnersByID возвращает владельцев из реестра, ключ — идентификатор.
// Нужен для сравнения состояний между снимками.
func (s *Identifiers) OwnersByID(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT edo_id, owner_raw FROM edo_identifiers`)
	if err != nil {
		return nil, fmt.Errorf("store: не прочитать владельцев: %w", err)
	}
	defer func() { _ = rows.Close() }()

	owners := make(map[string]string)
	for rows.Next() {
		var id, owner string
		if err := rows.Scan(&id, &owner); err != nil {
			return nil, fmt.Errorf("store: не разобрать владельца: %w", err)
		}
		owners[id] = owner
	}
	return owners, rows.Err()
}

// All возвращает весь реестр.
func (s *Identifiers) All(ctx context.Context) ([]IdentifierRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT edo_id, inn, kpp, client_name, login, owner_raw, owner_code,
		       its_tariffs, limit_docs, packets, first_seen_at, last_seen_at, last_period
		FROM edo_identifiers ORDER BY client_name, edo_id`)
	if err != nil {
		return nil, fmt.Errorf("store: не прочитать реестр: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []IdentifierRecord
	for rows.Next() {
		var r IdentifierRecord
		var limit sql.NullInt64
		var first, last int64
		if err := rows.Scan(&r.EDOID, &r.INN, &r.KPP, &r.ClientName, &r.Login,
			&r.OwnerRaw, &r.OwnerCode, &r.ITSTariffs, &limit, &r.Packets,
			&first, &last, &r.LastPeriod); err != nil {
			return nil, fmt.Errorf("store: не разобрать запись реестра: %w", err)
		}
		if limit.Valid {
			value := limit.Int64
			r.Limit = &value
		}
		r.FirstSeenAt = time.Unix(first, 0).UTC()
		r.LastSeenAt = time.Unix(last, 0).UTC()
		result = append(result, r)
	}
	return result, rows.Err()
}

// RecordEvent сохраняет находку. Повтор с тем же отпечатком состояния новой записи
// не создаёт: одна и та же аномалия не должна плодить записи при каждом снапшоте.
// Уверенность и подробности при этом обновляются — так подозрение «идентификатор
// исчез» становится подтверждённым, не меняя отпечатка.
func (s *Identifiers) RecordEvent(ctx context.Context, event AnomalyEvent, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO identifier_events (
			edo_id, kind, confidence, state_fingerprint, inn, kpp,
			client_name, login, details, detected_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(edo_id, kind, state_fingerprint) DO UPDATE SET
			confidence = excluded.confidence, details = excluded.details`,
		event.EDOID, event.Kind, event.Confidence, event.StateFingerprint,
		event.INN, event.KPP, event.ClientName, event.Login, event.Details,
		at.UTC().Unix())
	if err != nil {
		return fmt.Errorf("store: не записать находку по %s: %w", event.EDOID, err)
	}
	return nil
}

// Events возвращает находки. При includeAcknowledged = false подтверждённые скрываются.
func (s *Identifiers) Events(ctx context.Context, includeAcknowledged bool) ([]AnomalyEvent, error) {
	query := `
		SELECT e.id, e.edo_id, e.kind, e.confidence, e.state_fingerprint,
		       e.inn, e.kpp, e.client_name, e.login, e.details, e.detected_at,
		       a.edo_id IS NOT NULL AS acknowledged
		FROM identifier_events e
		LEFT JOIN identifier_acks a
			ON a.edo_id = e.edo_id AND a.state_fingerprint = e.state_fingerprint`
	if !includeAcknowledged {
		query += ` WHERE a.edo_id IS NULL`
	}
	query += ` ORDER BY e.detected_at DESC, e.id DESC`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("store: не прочитать находки: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []AnomalyEvent
	for rows.Next() {
		var e AnomalyEvent
		var detected int64
		if err := rows.Scan(&e.ID, &e.EDOID, &e.Kind, &e.Confidence, &e.StateFingerprint,
			&e.INN, &e.KPP, &e.ClientName, &e.Login, &e.Details, &detected,
			&e.Acknowledged); err != nil {
			return nil, fmt.Errorf("store: не разобрать находку: %w", err)
		}
		e.DetectedAt = time.Unix(detected, 0).UTC()
		result = append(result, e)
	}
	return result, rows.Err()
}

// Acked возвращает подтверждённые состояния, ключ — «идентификатор/отпечаток».
// Нужен находкам, которые не хранятся, а считаются на лету (прогноз лимита).
func (s *Identifiers) Acked(ctx context.Context) (map[string]bool, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT edo_id, state_fingerprint FROM identifier_acks`)
	if err != nil {
		return nil, fmt.Errorf("store: не прочитать подтверждения: %w", err)
	}
	defer func() { _ = rows.Close() }()

	acked := make(map[string]bool)
	for rows.Next() {
		var id, fp string
		if err := rows.Scan(&id, &fp); err != nil {
			return nil, fmt.Errorf("store: не разобрать подтверждение: %w", err)
		}
		acked[id+"/"+fp] = true
	}
	return acked, rows.Err()
}

// Acknowledge помечает состояние законным. Гасится только этот отпечаток:
// если состояние изменится, сигнал появится снова.
func (s *Identifiers) Acknowledge(
	ctx context.Context, edoID, fingerprint, reason, author string, at time.Time,
) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO identifier_acks (edo_id, state_fingerprint, reason, author, acked_at, review_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(edo_id, state_fingerprint) DO UPDATE SET
			reason = excluded.reason, author = excluded.author, acked_at = excluded.acked_at,
			review_at = excluded.review_at`,
		edoID, fingerprint, reason, author, at.UTC().Unix(), at.Add(AckReviewAfter).UTC().Unix())
	if err != nil {
		return fmt.Errorf("store: не сохранить подтверждение: %w", err)
	}
	return nil
}

// AckReviewAfter — через столько пометку «это законно» пора пересмотреть (спека §4.2).
const AckReviewAfter = 182 * 24 * time.Hour

// Ack — пометка «это законно» с автором, причиной и датой пересмотра.
type Ack struct {
	Reason   string
	Author   string
	AckedAt  time.Time
	ReviewAt time.Time
}

// Acks возвращает пометки, ключ — «идентификатор/отпечаток».
func (s *Identifiers) Acks(ctx context.Context) (map[string]Ack, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT edo_id, state_fingerprint, reason, author, acked_at, review_at FROM identifier_acks`)
	if err != nil {
		return nil, fmt.Errorf("store: не прочитать подтверждения: %w", err)
	}
	defer func() { _ = rows.Close() }()

	acks := make(map[string]Ack)
	for rows.Next() {
		var id, fp string
		var a Ack
		var acked, review int64
		if err := rows.Scan(&id, &fp, &a.Reason, &a.Author, &acked, &review); err != nil {
			return nil, fmt.Errorf("store: не разобрать подтверждение: %w", err)
		}
		a.AckedAt, a.ReviewAt = time.Unix(acked, 0).UTC(), time.Unix(review, 0).UTC()
		acks[id+"/"+fp] = a
	}
	return acks, rows.Err()
}

// Unacknowledge снимает пометку: находка снова видна среди активных.
func (s *Identifiers) Unacknowledge(ctx context.Context, edoID, fingerprint string) error {
	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM identifier_acks WHERE edo_id = ? AND state_fingerprint = ?`, edoID, fingerprint); err != nil {
		return fmt.Errorf("store: не снять подтверждение: %w", err)
	}
	return nil
}

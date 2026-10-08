package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// Sessions хранит сессии сотрудников. В базе лежит только хеш токена:
// утечка базы не даёт войти в сервис.
type Sessions struct {
	db *sql.DB
}

// NewSessions создаёт хранилище поверх открытой базы.
func NewSessions(db *sql.DB) *Sessions {
	return &Sessions{db: db}
}

// Create выдаёт новый токен сессии и возвращает его в открытом виде — единственный раз.
func (s *Sessions) Create(ctx context.Context, ttl time.Duration, ip, userAgent string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("store: не сгенерировать токен: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)

	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (token_hash, created_at, expires_at, ip, user_agent)
		 VALUES (?, ?, ?, ?, ?)`,
		hashToken(token), now.Unix(), now.Add(ttl).Unix(), ip, userAgent,
	)
	if err != nil {
		return "", fmt.Errorf("store: не сохранить сессию: %w", err)
	}
	return token, nil
}

// Validate сообщает, действует ли сессия. Ошибка возвращается только при сбое базы.
func (s *Sessions) Validate(ctx context.Context, token string) (bool, error) {
	var expiresAt int64
	err := s.db.QueryRowContext(ctx,
		`SELECT expires_at FROM sessions WHERE token_hash = ?`, hashToken(token),
	).Scan(&expiresAt)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("store: не проверить сессию: %w", err)
	}
	return time.Now().UTC().Unix() < expiresAt, nil
}

// Extend проверяет сессию и сдвигает её срок на now+ttl (скользящий срок).
// Чтобы не писать в базу на каждый запрос, срок сдвигается, только когда до
// конца осталось меньше ttl-every, то есть не чаще раза в every.
// ok — сессия действует; extended — срок только что сдвинут, и cookie пора
// перевыпустить. Ошибка возвращается только при сбое базы.
func (s *Sessions) Extend(ctx context.Context, token string, ttl, every time.Duration) (ok, extended bool, err error) {
	hash := hashToken(token)
	var expiresAt int64
	err = s.db.QueryRowContext(ctx,
		`SELECT expires_at FROM sessions WHERE token_hash = ?`, hash,
	).Scan(&expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return false, false, nil
	}
	if err != nil {
		return false, false, fmt.Errorf("store: не проверить сессию: %w", err)
	}
	now := time.Now().UTC()
	if now.Unix() >= expiresAt {
		return false, false, nil
	}
	if time.Unix(expiresAt, 0).Sub(now) >= ttl-every {
		return true, false, nil
	}
	_, err = s.db.ExecContext(ctx,
		`UPDATE sessions SET expires_at = ? WHERE token_hash = ? AND expires_at > ?`,
		now.Add(ttl).Unix(), hash, now.Unix(),
	)
	if err != nil {
		return false, false, fmt.Errorf("store: не продлить сессию: %w", err)
	}
	return true, true, nil
}

// Delete завершает сессию.
func (s *Sessions) Delete(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, hashToken(token))
	if err != nil {
		return fmt.Errorf("store: не удалить сессию: %w", err)
	}
	return nil
}

// PurgeExpired удаляет истёкшие сессии и возвращает их количество.
func (s *Sessions) PurgeExpired(ctx context.Context) (int64, error) {
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM sessions WHERE expires_at <= ?`, time.Now().UTC().Unix())
	if err != nil {
		return 0, fmt.Errorf("store: не очистить сессии: %w", err)
	}
	return result.RowsAffected()
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

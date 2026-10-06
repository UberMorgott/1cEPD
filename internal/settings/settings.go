// Package settings хранит настройки отправки почты. Пароль SMTP лежит
// зашифрованным и наружу не отдаётся: его читает только отправщик писем.
package settings

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// Settings — настройки почты без пароля.
type Settings struct {
	SMTPHost  string
	SMTPPort  int
	SMTPLogin string
	SMTPFrom  string
	MailTo    []string
	// PasswordSet говорит, что пароль сохранён, не раскрывая его.
	PasswordSet bool
	UpdatedAt   time.Time
}

// Store читает и пишет единственную строку настроек.
type Store struct {
	db  *sql.DB
	key []byte
}

// New создаёт хранилище поверх открытой базы и ключа шифрования.
func New(db *sql.DB, key []byte) *Store {
	return &Store{db: db, key: key}
}

// Get возвращает настройки. Пароль сюда не попадает.
func (s *Store) Get(ctx context.Context) (Settings, error) {
	current, blob, err := s.read(ctx)
	if err != nil {
		return Settings{}, err
	}
	if len(blob) > 0 {
		// Потерянный или подменённый ключ не должен ронять сервис: настройки
		// открываются, пароль просто считается незаданным.
		if _, err := decrypt(s.key, blob); err != nil {
			slog.Warn("сохранённый пароль SMTP не расшифровывается, считаем его незаданным", "err", err)
		} else {
			current.PasswordSet = true
		}
	}
	return current, nil
}

// Password возвращает пароль в открытом виде. Нужен только отправщику писем.
// Пустая строка означает, что пароля нет.
func (s *Store) Password(ctx context.Context) (string, error) {
	_, blob, err := s.read(ctx)
	if err != nil {
		return "", err
	}
	if len(blob) == 0 {
		return "", nil
	}
	password, err := decrypt(s.key, blob)
	if err != nil {
		slog.Warn("сохранённый пароль SMTP не расшифровывается, считаем его незаданным", "err", err)
		return "", nil
	}
	return password, nil
}

// Save перезаписывает настройки.
//
// password задаёт судьбу пароля: nil — оставить прежний, пустая строка —
// удалить, непустая — заменить.
func (s *Store) Save(ctx context.Context, in Settings, password *string, at time.Time) error {
	mailTo, err := json.Marshal(nonNil(in.MailTo))
	if err != nil {
		return fmt.Errorf("settings: не сохранить список получателей: %w", err)
	}

	query := `UPDATE settings
		SET smtp_host = ?, smtp_port = ?, smtp_login = ?, smtp_from = ?, mail_to = ?, updated_at = ?
		WHERE id = 1`
	args := []any{in.SMTPHost, in.SMTPPort, in.SMTPLogin, in.SMTPFrom, string(mailTo), at.UTC().Unix()}

	if password != nil {
		var blob []byte
		if *password != "" {
			if blob, err = encrypt(s.key, *password); err != nil {
				return err
			}
		}
		query = `UPDATE settings
			SET smtp_host = ?, smtp_port = ?, smtp_login = ?, smtp_from = ?, mail_to = ?, updated_at = ?,
			    smtp_password_enc = ?
			WHERE id = 1`
		args = append(args, blob)
	}

	if _, err := s.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("settings: не сохранить настройки: %w", err)
	}
	return nil
}

func (s *Store) read(ctx context.Context) (Settings, []byte, error) {
	var current Settings
	var mailTo string
	var updated int64
	var blob []byte

	err := s.db.QueryRowContext(ctx, `
		SELECT smtp_host, smtp_port, smtp_login, smtp_from, mail_to, updated_at, smtp_password_enc
		FROM settings WHERE id = 1`).
		Scan(&current.SMTPHost, &current.SMTPPort, &current.SMTPLogin, &current.SMTPFrom,
			&mailTo, &updated, &blob)
	if err != nil {
		return Settings{}, nil, fmt.Errorf("settings: не прочитать настройки: %w", err)
	}

	if err := json.Unmarshal([]byte(mailTo), &current.MailTo); err != nil {
		return Settings{}, nil, fmt.Errorf("settings: список получателей повреждён: %w", err)
	}
	current.MailTo = nonNil(current.MailTo)
	current.UpdatedAt = time.Unix(updated, 0).UTC()
	return current, blob, nil
}

// nonNil бережёт JSON-ответ: nil-срез уехал бы клиенту как null вместо [].
func nonNil(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}

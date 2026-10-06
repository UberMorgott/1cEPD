package settings

import (
	"context"
	"fmt"
	"log/slog"
)

// Sender — отправитель заявки ИТС: шапка формы, которая одинакова во всех
// заявках партнёра. Password — пароль подтверждения подлинности заявки, секрет:
// в лог не пишется.
type Sender struct {
	PartnerCode string
	Responsible string
	Email       string
	Password    string
}

// Sender возвращает отправителя заявки вместе с паролем в открытом виде:
// форма заявки подставляет его сама. Нерасшифровываемый пароль считается пустым.
func (s *Store) Sender(ctx context.Context) (Sender, error) {
	var sender Sender
	var blob []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT its_partner_code, its_responsible, its_email, its_password_enc
		FROM settings WHERE id = 1`).
		Scan(&sender.PartnerCode, &sender.Responsible, &sender.Email, &blob)
	if err != nil {
		return Sender{}, fmt.Errorf("settings: не прочитать отправителя заявки: %w", err)
	}
	if len(blob) > 0 {
		password, err := decrypt(s.key, blob)
		if err != nil {
			slog.Warn("сохранённый пароль заявки не расшифровывается, считаем его незаданным", "err", err)
		} else {
			sender.Password = password
		}
	}
	return sender, nil
}

// SaveSender перезаписывает отправителя заявки. Пустой пароль удаляет сохранённый.
func (s *Store) SaveSender(ctx context.Context, sender Sender) error {
	var blob []byte
	if sender.Password != "" {
		var err error
		if blob, err = encrypt(s.key, sender.Password); err != nil {
			return err
		}
	}
	_, err := s.db.ExecContext(ctx, `UPDATE settings
		SET its_partner_code = ?, its_responsible = ?, its_email = ?, its_password_enc = ?
		WHERE id = 1`,
		sender.PartnerCode, sender.Responsible, sender.Email, blob)
	if err != nil {
		return fmt.Errorf("settings: не сохранить отправителя заявки: %w", err)
	}
	return nil
}

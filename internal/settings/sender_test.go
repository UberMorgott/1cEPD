package settings_test

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"partnerops/internal/settings"
	"partnerops/internal/store"
)

func openStore(t *testing.T) (*settings.Store, func(query string) []byte) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	key, err := settings.LoadKey(dbPath)
	if err != nil {
		t.Fatalf("LoadKey: %v", err)
	}
	raw := func(query string) []byte {
		var blob []byte
		if err := db.QueryRow(query).Scan(&blob); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		return blob
	}
	return settings.New(db, key), raw
}

func TestSenderEmptyByDefault(t *testing.T) {
	s, _ := openStore(t)
	got, err := s.Sender(context.Background())
	if err != nil {
		t.Fatalf("Sender: %v", err)
	}
	if got != (settings.Sender{}) {
		t.Fatalf("по умолчанию отправитель пуст, а получили %+v", got)
	}
}

func TestSenderRoundTrip(t *testing.T) {
	s, raw := openStore(t)
	ctx := context.Background()
	want := settings.Sender{ //nolint:gosec // тестовый пароль, не настоящий
		PartnerCode: "00000", Responsible: "Иванов И. И.",
		Email: "its@example.ru", Password: "секрет-42",
	}
	if err := s.SaveSender(ctx, want); err != nil {
		t.Fatalf("SaveSender: %v", err)
	}
	got, err := s.Sender(ctx)
	if err != nil {
		t.Fatalf("Sender: %v", err)
	}
	if got != want {
		t.Fatalf("Sender = %+v, want %+v", got, want)
	}
	if blob := raw(`SELECT its_password_enc FROM settings WHERE id = 1`); bytes.Contains(blob, []byte(want.Password)) {
		t.Fatal("пароль заявки лежит в базе открытым текстом")
	}

	// Пустой пароль удаляет сохранённый, остальное перезаписывается.
	cleared := settings.Sender{PartnerCode: "1", Responsible: "", Email: "", Password: ""}
	if err := s.SaveSender(ctx, cleared); err != nil {
		t.Fatalf("SaveSender: %v", err)
	}
	if got, _ = s.Sender(ctx); got != cleared {
		t.Fatalf("после очистки Sender = %+v, want %+v", got, cleared)
	}
}

func TestSaveSenderKeepsMailSettings(t *testing.T) {
	s, _ := openStore(t)
	ctx := context.Background()
	before, err := s.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if err := s.SaveSender(ctx, settings.Sender{PartnerCode: "00000"}); err != nil {
		t.Fatalf("SaveSender: %v", err)
	}
	after, err := s.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if after.SMTPHost != before.SMTPHost || len(after.MailTo) != len(before.MailTo) {
		t.Fatalf("настройки почты изменились: %+v -> %+v", before, after)
	}
}

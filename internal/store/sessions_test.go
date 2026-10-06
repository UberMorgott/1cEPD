package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func testSessions(t *testing.T) *Sessions {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewSessions(db)
}

func TestCreateAndValidate(t *testing.T) {
	s := testSessions(t)
	ctx := context.Background()

	token, err := s.Create(ctx, time.Hour, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(token) < 32 {
		t.Errorf("токен подозрительно короткий: %d символов", len(token))
	}

	ok, err := s.Validate(ctx, token)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !ok {
		t.Error("свежая сессия не прошла проверку")
	}
}

func TestValidateRejectsUnknownToken(t *testing.T) {
	s := testSessions(t)
	ok, err := s.Validate(context.Background(), "не существует")
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if ok {
		t.Error("неизвестный токен прошёл проверку")
	}
}

func TestValidateRejectsExpired(t *testing.T) {
	s := testSessions(t)
	ctx := context.Background()

	token, err := s.Create(ctx, -time.Minute, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	ok, err := s.Validate(ctx, token)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if ok {
		t.Error("просроченная сессия прошла проверку")
	}
}

func TestDeleteRemovesSession(t *testing.T) {
	s := testSessions(t)
	ctx := context.Background()

	token, _ := s.Create(ctx, time.Hour, "127.0.0.1", "agent")
	if err := s.Delete(ctx, token); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	ok, _ := s.Validate(ctx, token)
	if ok {
		t.Error("удалённая сессия прошла проверку")
	}
}

func TestTokensAreUnique(t *testing.T) {
	s := testSessions(t)
	ctx := context.Background()
	seen := make(map[string]bool)
	for range 50 {
		token, err := s.Create(ctx, time.Hour, "", "")
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if seen[token] {
			t.Fatal("токен повторился")
		}
		seen[token] = true
	}
}

func TestPurgeExpiredRemovesOnlyStale(t *testing.T) {
	s := testSessions(t)
	ctx := context.Background()

	fresh, _ := s.Create(ctx, time.Hour, "", "")
	if _, err := s.Create(ctx, -time.Hour, "", ""); err != nil {
		t.Fatalf("Create: %v", err)
	}

	removed, err := s.PurgeExpired(ctx)
	if err != nil {
		t.Fatalf("PurgeExpired: %v", err)
	}
	if removed != 1 {
		t.Errorf("удалено %d сессий, ожидали 1", removed)
	}
	if ok, _ := s.Validate(ctx, fresh); !ok {
		t.Error("живая сессия была удалена")
	}
}

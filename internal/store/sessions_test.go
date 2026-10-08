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

func sessionExpiry(t *testing.T, s *Sessions, token string) int64 {
	t.Helper()
	var exp int64
	if err := s.db.QueryRowContext(context.Background(),
		`SELECT expires_at FROM sessions WHERE token_hash = ?`, hashToken(token)).Scan(&exp); err != nil {
		t.Fatalf("expires_at: %v", err)
	}
	return exp
}

func TestExtendSlidesExpiry(t *testing.T) {
	s := testSessions(t)
	ctx := context.Background()

	// До конца 30 минут из часа: продление уже положено.
	token, err := s.Create(ctx, 30*time.Minute, "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	ok, extended, err := s.Extend(ctx, token, time.Hour, 10*time.Minute)
	if err != nil {
		t.Fatalf("Extend: %v", err)
	}
	if !ok || !extended {
		t.Fatalf("ok=%v extended=%v, ожидали продление", ok, extended)
	}
	if got, want := sessionExpiry(t, s, token), time.Now().Add(time.Hour).Unix(); got < want-5 || got > want+5 {
		t.Errorf("expires_at = %d, ожидали около %d", got, want)
	}
}

func TestExtendThrottlesWrites(t *testing.T) {
	s := testSessions(t)
	ctx := context.Background()

	// Сессия только что выдана на час: следующие 10 минут в базу не пишем.
	token, err := s.Create(ctx, time.Hour, "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	before := sessionExpiry(t, s, token)
	ok, extended, err := s.Extend(ctx, token, time.Hour, 10*time.Minute)
	if err != nil {
		t.Fatalf("Extend: %v", err)
	}
	if !ok || extended {
		t.Fatalf("ok=%v extended=%v, ожидали действующую сессию без записи", ok, extended)
	}
	if after := sessionExpiry(t, s, token); after != before {
		t.Errorf("expires_at изменился: %d -> %d", before, after)
	}
}

func TestExtendRejectsExpiredAndUnknown(t *testing.T) {
	s := testSessions(t)
	ctx := context.Background()

	token, err := s.Create(ctx, -time.Minute, "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	for _, tok := range []string{token, "не существует"} {
		ok, extended, err := s.Extend(ctx, tok, time.Hour, 10*time.Minute)
		if err != nil {
			t.Fatalf("Extend: %v", err)
		}
		if ok || extended {
			t.Errorf("токен %q: ok=%v extended=%v, ожидали отказ", tok, ok, extended)
		}
	}
	// Истёкшая сессия не воскресает.
	if exp := sessionExpiry(t, s, token); exp > time.Now().Unix() {
		t.Errorf("истёкшая сессия продлена до %d", exp)
	}
}

package httpapi

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"partnerops/internal/settings"
	"partnerops/internal/store"
)

func testSettingsStore(t *testing.T) *settings.Store {
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
	return settings.New(db, key)
}

// Свежая база не знает никакого SMTP-сервера: сервис не ходит в сеть,
// пока пользователь сам не укажет почтовый сервер.
func TestFreshSettingsHaveNoSMTPHost(t *testing.T) {
	current, err := testSettingsStore(t).Get(t.Context())
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if current.SMTPHost != "" {
		t.Errorf("SMTPHost = %q, ожидали пустой", current.SMTPHost)
	}
}

func TestTestSMTPWithoutServerReportsNotConfigured(t *testing.T) {
	h := NewSettings(testSettingsStore(t))
	rec := httptest.NewRecorder()
	h.TestSMTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/settings/smtp/test", nil))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("код %d, ожидали 400", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "not_configured") || !strings.Contains(body, "Почтовый сервер не настроен") {
		t.Errorf("тело %s, ожидали not_configured и «Почтовый сервер не настроен»", body)
	}
}

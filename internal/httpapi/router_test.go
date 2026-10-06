package httpapi

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"partnerops/internal/events"
	"partnerops/internal/store"
)

func testRouter(t *testing.T) http.Handler {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	hash, _ := bcrypt.GenerateFromPassword([]byte("пароль"), bcrypt.DefaultCost)
	auth := NewAuth(store.NewSessions(db), "admin", string(hash), time.Hour, true)
	return NewRouter(auth, NewEvents(events.NewBus()), nil, nil, nil, nil, nil, "")
}

func TestHealthIsPublic(t *testing.T) {
	rec := httptest.NewRecorder()
	testRouter(t).ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/health", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("код %d, ожидали 200", rec.Code)
	}
}

func TestEventsRequireSession(t *testing.T) {
	rec := httptest.NewRecorder()
	testRouter(t).ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/events", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("код %d, ожидали 401", rec.Code)
	}
}

func TestUnknownPathIsNotFound(t *testing.T) {
	rec := httptest.NewRecorder()
	testRouter(t).ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/nope", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("код %d, ожидали 404", rec.Code)
	}
}

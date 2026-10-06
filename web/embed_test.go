package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlerServesIndex(t *testing.T) {
	handler, err := Handler()
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("код %d, ожидали 200", rec.Code)
	}
	if rec.Body.Len() == 0 {
		t.Error("пустой ответ")
	}
}

func TestHandlerFallsBackToIndex(t *testing.T) {
	// Маршруты Vue Router обрабатываются на клиенте: любой неизвестный путь
	// должен отдавать index.html, иначе перезагрузка страницы даст 404 —
	// например, на карточке клиента.
	handler, err := Handler()
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/clients/7700000011-770001001", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("код %d, ожидали 200 с index.html", rec.Code)
	}
}

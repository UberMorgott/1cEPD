package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestSecurityHeadersSet(t *testing.T) {
	rec := httptest.NewRecorder()
	SecurityHeaders(okHandler()).ServeHTTP(rec,
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	for name, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"Referrer-Policy":        "no-referrer",
		"X-Frame-Options":        "DENY",
	} {
		if got := rec.Header().Get(name); got != want {
			t.Errorf("%s = %q, ожидали %q", name, got, want)
		}
	}
	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Error("Content-Security-Policy не выставлен")
	}
	// Приложение наружу не ходит: шрифт вшит в dist, чужих источников быть не должно.
	if strings.Contains(csp, "http") {
		t.Errorf("в CSP остался внешний источник: %q", csp)
	}
}

func TestRejectCrossSite(t *testing.T) {
	cases := []struct {
		name   string
		method string
		site   string
		want   int
	}{
		{"свой запрос", http.MethodPost, "same-origin", http.StatusOK},
		{"адресная строка", http.MethodPost, "none", http.StatusOK},
		{"без заголовка", http.MethodPost, "", http.StatusOK},
		{"чужая страница", http.MethodPost, "cross-site", http.StatusForbidden},
		{"соседний порт", http.MethodPost, "same-site", http.StatusForbidden},
		{"чтение не трогаем", http.MethodGet, "cross-site", http.StatusOK},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), c.method, "/api/login", nil)
			if c.site != "" {
				req.Header.Set("Sec-Fetch-Site", c.site)
			}
			rec := httptest.NewRecorder()
			RejectCrossSite(okHandler()).ServeHTTP(rec, req)

			if rec.Code != c.want {
				t.Errorf("код %d, ожидали %d", rec.Code, c.want)
			}
		})
	}
}

package partner

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestClientSendsBasicAuth(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "api-login-00000", "secret")
	if _, err := c.post(context.Background(), "/anything", []byte(`{}`)); err != nil {
		t.Fatalf("post вернул ошибку: %v", err)
	}

	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("api-login-00000:secret"))
	if gotAuth != want {
		t.Errorf("Authorization = %q, ожидали %q", gotAuth, want)
	}
}

func TestClientReportsUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"status":401,"message":"Bad credentials"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "login", "wrong")
	_, err := c.post(context.Background(), "/anything", []byte(`{}`))
	if err == nil {
		t.Fatal("ожидали ошибку авторизации")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("ошибка %q не содержит код 401", err.Error())
	}
}

func TestClientReportsRateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"MAX_TASKS_PER_HOUR_LIMIT_REACHED"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "login", "secret")
	_, err := c.post(context.Background(), "/anything", []byte(`{}`))
	if !IsRateLimited(err) {
		t.Errorf("IsRateLimited(%v) = false, ожидали true", err)
	}
}

// Тело ответа не должно читаться целиком: сервер может отдать сколь угодно много.
func TestClientLimitsResponseSize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(bytes.Repeat([]byte("x"), 4096))
	}))
	defer srv.Close()

	c := New(srv.URL, "login", "secret")
	c.maxResponseBytes = 1024

	if _, err := c.get(context.Background(), "/huge"); err == nil {
		t.Fatal("ожидали ошибку о превышении лимита размера ответа")
	}
}

// Редирект уводит Authorization на чужой хост и превращает POST в GET.
func TestClientRefusesRedirect(t *testing.T) {
	var reached atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached.Add(1)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer target.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/anything", http.StatusFound)
	}))
	defer srv.Close()

	c := New(srv.URL, "login", "secret")
	if _, err := c.post(context.Background(), "/anything", []byte(`{}`)); err == nil {
		t.Fatal("ожидали ошибку о запрещённом редиректе")
	}
	if n := reached.Load(); n != 0 {
		t.Errorf("запросов на чужой хост %d, ожидали 0", n)
	}
}

// Spring кладёт в поле error текст статуса, иногда в виде BAD_REQUEST —
// это не прикладной код, и путать его с кодом нельзя.
func TestAPIErrorIgnoresStatusTextCode(t *testing.T) {
	for _, body := range []string{`{"error":"BAD_REQUEST"}`, `{"error":"Bad Request"}`} {
		err := apiErrorFrom(http.StatusBadRequest, []byte(body))
		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("apiErrorFrom вернул %T", err)
		}
		if apiErr.Code != "" {
			t.Errorf("Code = %q для %s, ожидали пустой", apiErr.Code, body)
		}
	}

	err := apiErrorFrom(http.StatusBadRequest, []byte(`{"error":"MAX_TASKS_PER_HOUR_LIMIT_REACHED"}`))
	if !IsRateLimited(err) {
		t.Errorf("IsRateLimited(%v) = false, ожидали true", err)
	}
}

func TestClientRespectsContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	c := New(srv.URL, "login", "secret")
	if _, err := c.post(ctx, "/slow", []byte(`{}`)); err == nil {
		t.Fatal("ожидали ошибку по истечении контекста")
	}
}

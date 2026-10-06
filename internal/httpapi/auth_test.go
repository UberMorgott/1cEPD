package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"partnerops/internal/store"
)

func testAuth(t *testing.T) *Auth {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	// MinCost: тестам нужна проверка пароля, а не его стоимость.
	hash, err := bcrypt.GenerateFromPassword([]byte("пароль"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	auth := NewAuth(store.NewSessions(db), "admin", string(hash), time.Hour, true)
	fastDelays(auth.limiter)
	return auth
}

func TestLoginSucceeds(t *testing.T) {
	auth := testAuth(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/login",
		strings.NewReader(`{"login":"admin","password":"пароль"}`))
	rec := httptest.NewRecorder()
	auth.Login(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("код %d, ожидали 200, тело: %s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("cookie сессии не выставлена")
	}
	c := cookies[0]
	if !c.HttpOnly {
		t.Error("cookie должна быть HttpOnly")
	}
	if c.SameSite != http.SameSiteStrictMode {
		t.Error("cookie должна быть SameSite=Strict")
	}
	if !c.Secure {
		t.Error("при COOKIE_SECURE=true cookie должна быть Secure")
	}
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	auth := testAuth(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/login",
		strings.NewReader(`{"login":"admin","password":"неверный"}`))
	rec := httptest.NewRecorder()
	auth.Login(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("код %d, ожидали 401", rec.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if _, ok := body["error"]; !ok {
		t.Error("в теле нет поля error")
	}
}

func TestLoginRateLimited(t *testing.T) {
	auth := testAuth(t)

	for range 5 {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/login",
			strings.NewReader(`{"login":"admin","password":"неверный"}`))
		req.RemoteAddr = "10.0.0.1:1234"
		auth.Login(httptest.NewRecorder(), req)
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/login",
		strings.NewReader(`{"login":"admin","password":"пароль"}`))
	req.RemoteAddr = "10.0.0.1:1234"
	rec := httptest.NewRecorder()
	auth.Login(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("код %d, ожидали 429", rec.Code)
	}
}

// Блокировка должна не только срабатывать, но и отпускать.
func TestLoginLockoutExpires(t *testing.T) {
	auth := testAuth(t)
	auth.limiter = newLimiter(2, 100*time.Millisecond)
	fastDelays(auth.limiter)

	attempt := func(password string) int {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/login",
			strings.NewReader(`{"login":"admin","password":"`+password+`"}`))
		req.RemoteAddr = "10.0.0.7:1234"
		rec := httptest.NewRecorder()
		auth.Login(rec, req)
		return rec.Code
	}

	attempt("неверный")
	attempt("неверный")
	if code := attempt("пароль"); code != http.StatusTooManyRequests {
		t.Fatalf("код %d, ожидали 429 сразу после лимита", code)
	}

	time.Sleep(150 * time.Millisecond)
	if code := attempt("пароль"); code != http.StatusOK {
		t.Errorf("код %d, ожидали 200 после окончания блокировки", code)
	}
}

// Неизвестный логин и неверный пароль обязаны отвечать неотличимо,
// иначе перебором находится настоящий логин.
func TestLoginHidesWhetherLoginExists(t *testing.T) {
	auth := testAuth(t)

	answer := func(body string) (int, string) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/login",
			strings.NewReader(body))
		req.RemoteAddr = "10.0.0.8:1234"
		rec := httptest.NewRecorder()
		auth.Login(rec, req)
		return rec.Code, rec.Body.String()
	}

	unknownCode, unknownBody := answer(`{"login":"нет-такого","password":"пароль"}`)
	wrongCode, wrongBody := answer(`{"login":"admin","password":"неверный"}`)

	if unknownCode != wrongCode {
		t.Errorf("коды разошлись: %d и %d", unknownCode, wrongCode)
	}
	if unknownBody != wrongBody {
		t.Errorf("тела разошлись: %q и %q", unknownBody, wrongBody)
	}
}

// Задержка не должна стать новым оракулом: неизвестный логин копит неудачи и
// ждёт ровно столько же, сколько настоящий.
func TestLoginDelaysUnknownLoginLikeKnown(t *testing.T) {
	auth := testAuth(t)
	auth.limiter.delayUnit = 20 * time.Millisecond
	auth.limiter.delayCap = time.Second

	// Каждый логин пробуем со своего адреса: жёсткая блокировка по адресу осталась.
	measure := func(ip, login string) time.Duration {
		attempt := func() {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/login",
				strings.NewReader(`{"login":"`+login+`","password":"неверный"}`))
			req.RemoteAddr = ip + ":1234"
			auth.Login(httptest.NewRecorder(), req)
		}
		for range 3 {
			attempt()
		}
		start := time.Now()
		attempt() // четвёртая подряд неудача — восемь единиц
		return time.Since(start)
	}

	known := measure("10.0.0.21", "admin")
	unknown := measure("10.0.0.22", "нет-такого")

	want := 8 * auth.limiter.delayUnit
	for name, got := range map[string]time.Duration{"известный": known, "неизвестный": unknown} {
		if got < want || got > 2*want {
			t.Errorf("%s логин ждал %v, ожидали около %v", name, got, want)
		}
	}
}

func TestLogoutInvalidatesSession(t *testing.T) {
	auth := testAuth(t)

	token, err := auth.sessions.Create(t.Context(), time.Hour, "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	out := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/logout", nil)
	//nolint:gosec // cookie уходит в запросе: браузер шлёт только имя и значение, атрибуты здесь бессмысленны
	out.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	auth.Logout(httptest.NewRecorder(), out)

	handler := auth.RequireSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("обработчик вызван с погашенной сессией")
	}))
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/whatever", nil)
	//nolint:gosec // cookie уходит в запросе: браузер шлёт только имя и значение, атрибуты здесь бессмысленны
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("код %d, ожидали 401 после выхода", rec.Code)
	}
}

// Токен, предъявленный до входа, после входа работать не должен.
func TestLoginRotatesSession(t *testing.T) {
	auth := testAuth(t)

	old, err := auth.sessions.Create(t.Context(), time.Hour, "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/login",
		strings.NewReader(`{"login":"admin","password":"пароль"}`))
	//nolint:gosec // cookie уходит в запросе: браузер шлёт только имя и значение, атрибуты здесь бессмысленны
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: old})
	rec := httptest.NewRecorder()
	auth.Login(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("код %d, ожидали 200, тело: %s", rec.Code, rec.Body.String())
	}
	if ok, err := auth.sessions.Validate(t.Context(), old); err != nil || ok {
		t.Errorf("прежний токен всё ещё действует (ok=%v, err=%v)", ok, err)
	}
	fresh := rec.Result().Cookies()[0].Value
	if fresh == old {
		t.Error("выдан тот же токен, ожидали новый")
	}
}

func TestRequireSessionBlocksAnonymous(t *testing.T) {
	auth := testAuth(t)
	handler := auth.RequireSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/whatever", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("код %d, ожидали 401", rec.Code)
	}
}

func TestRequireSessionAllowsValidCookie(t *testing.T) {
	auth := testAuth(t)

	token, err := auth.sessions.Create(context.Background(), time.Hour, "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	called := false
	login := ""
	handler := auth.RequireSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		login = sessionLogin(r.Context())
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/whatever", nil)
	//nolint:gosec // cookie уходит в запросе: браузер шлёт только имя и значение, атрибуты здесь бессмысленны
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if !called {
		t.Error("обработчик не вызван при действующей сессии")
	}
	// Пометки подписываются этим логином, а не зашитым «admin».
	if login != "admin" {
		t.Errorf("логин сессии %q, ожидали логин из настроек", login)
	}
}

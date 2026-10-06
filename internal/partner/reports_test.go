package partner

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestEDOBillingReportPollsUntilReady(t *testing.T) {
	var polls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/edo/reports/billing":
			_, _ = w.Write([]byte(`{"taskUeid":"task-1","error":null}`))
		case r.Method == http.MethodGet && r.URL.Path == "/edo/reports/billing/task-1":
			if atomic.AddInt32(&polls, 1) < 3 {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			_, _ = w.Write([]byte("\ufeffКолонка\r\nзначение\r\n"))
		default:
			t.Errorf("неожиданный запрос %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	c := New(srv.URL, "login", "secret")
	c.pollInterval = time.Millisecond

	raw, err := c.EDOBillingReport(context.Background(), time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("EDOBillingReport вернул ошибку: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("отчёт пуст")
	}
	if atomic.LoadInt32(&polls) != 3 {
		t.Errorf("опросов %d, ожидали 3", polls)
	}
}

func TestEDOBillingReportFailsOnTaskError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"taskUeid":"","error":"что-то пошло не так"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "login", "secret")
	_, err := c.EDOBillingReport(context.Background(), time.Now())
	if err == nil {
		t.Fatal("ожидали ошибку из поля error")
	}
}

// Пустое тело при 200 — это не «отчёт ещё строится», а сломанный ответ:
// цикл опроса не должен крутиться на нём вечно.
func TestEDOBillingReportFailsOnEmptyBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"taskUeid":"task-1"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(srv.URL, "login", "secret")
	c.pollInterval = time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	_, err := c.EDOBillingReport(ctx, time.Now())
	if err == nil {
		t.Fatal("ожидали ошибку о пустом отчёте")
	}
	if ctx.Err() != nil {
		t.Error("вышли по таймауту контекста, а не по ошибке — цикл опроса вечный")
	}
}

// Любой код кроме 200 и 204 — ошибка, а не CSV.
func TestEDOBillingReportFailsOnUnexpectedStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"taskUeid":"task-1"}`))
			return
		}
		// 202 и 204 означают «отчёт ещё строится», поэтому для проверки
		// неожиданного кода берём тот, который 1С не использует.
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("не отчёт"))
	}))
	defer srv.Close()

	c := New(srv.URL, "login", "secret")
	c.pollInterval = time.Millisecond

	if _, err := c.EDOBillingReport(context.Background(), time.Now()); err == nil {
		t.Fatal("ожидали ошибку о неожиданном коде ответа")
	}
}

// Пауза между опросами отсчитывается после ответа: иначе долгий запрос
// «съедает» интервал и следующий опрос уходит немедленно.
func TestWaitForReportPausesAfterResponse(t *testing.T) {
	const pause = 40 * time.Millisecond

	var mu sync.Mutex
	var gets []time.Time
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gets = append(gets, time.Now())
		n := len(gets)
		mu.Unlock()

		if n == 1 {
			// Первый запрос длится дольше интервала опроса.
			time.Sleep(3 * pause)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_, _ = w.Write([]byte("отчёт"))
	}))
	defer srv.Close()

	c := New(srv.URL, "login", "secret")
	c.pollInterval = pause

	if _, err := c.waitForReport(context.Background(), "/report"); err != nil {
		t.Fatalf("waitForReport вернул ошибку: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(gets) != 2 {
		t.Fatalf("запросов %d, ожидали 2", len(gets))
	}
	// Между ответом на первый запрос и вторым запросом должна пройти пауза.
	if gap := gets[1].Sub(gets[0]) - 3*pause; gap < pause/2 {
		t.Errorf("пауза после ответа %v, ожидали не меньше %v", gap, pause)
	}
}

func TestEDOBillingReportStopsOnContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"taskUeid":"task-1"}`))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New(srv.URL, "login", "secret")
	c.pollInterval = time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	if _, err := c.EDOBillingReport(ctx, time.Now()); err == nil {
		t.Fatal("ожидали ошибку по истечении контекста")
	}
}

func TestEDOBillingReportWaitsOnAccepted(t *testing.T) {
	// Проверено живыми вызовами: пока отчёт строится, 1С отвечает не только 204,
	// но и 202. Трактовка 202 как ошибки ломает работу с настоящим API.
	var polls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"taskUeid":"task-accepted","error":null}`))
			return
		}
		if polls.Add(1) < 3 {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		_, _ = w.Write([]byte("\ufeffКолонка\r\nзначение\r\n"))
	}))
	defer srv.Close()

	c := New(srv.URL, "login", "secret")
	c.pollInterval = time.Millisecond

	raw, err := c.EDOBillingReport(context.Background(), time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("202 должен означать «ещё строится», получили ошибку: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("отчёт пуст")
	}
	if got := polls.Load(); got != 3 {
		t.Errorf("опросов %d, ожидали 3", got)
	}
}

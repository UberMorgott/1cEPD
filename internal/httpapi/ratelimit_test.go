package httpapi

import (
	"context"
	"strconv"
	"testing"
	"time"
)

// fastDelays ужимает прогрессию задержки: тестам нужен её порядок, а не секунды.
func fastDelays(l *limiter) {
	l.delayUnit = time.Millisecond
	l.delayCap = 8 * time.Millisecond
}

func TestLimiterBlocksAfterLimit(t *testing.T) {
	l := newLimiter(3, time.Minute)
	for i := range 3 {
		if l.blocked("1.2.3.4") {
			t.Fatalf("попытка %d заблокирована, ожидали разрешение", i+1)
		}
		l.fail("1.2.3.4")
	}
	if !l.blocked("1.2.3.4") {
		t.Error("четвёртая попытка разрешена, ожидали блокировку")
	}
}

func TestLimiterSeparatesKeys(t *testing.T) {
	l := newLimiter(1, time.Minute)
	l.fail("1.1.1.1")
	if !l.blocked("1.1.1.1") {
		t.Fatal("первый адрес не заблокирован")
	}
	if l.blocked("2.2.2.2") {
		t.Error("второй адрес заблокирован, хотя лимит у каждого свой")
	}
}

func TestLimiterForgetsAfterWindow(t *testing.T) {
	l := newLimiter(1, 10*time.Millisecond)
	l.fail("1.1.1.1")
	time.Sleep(20 * time.Millisecond)
	if l.blocked("1.1.1.1") {
		t.Error("после окончания окна попытка должна снова разрешаться")
	}
}

func TestLimiterResetClearsFailures(t *testing.T) {
	l := newLimiter(1, time.Minute)
	l.fail("1.1.1.1", "login:admin")
	l.reset("1.1.1.1", "login:admin")
	if l.blocked("1.1.1.1", "login:admin") {
		t.Error("после reset блокировки быть не должно")
	}
}

// Логин в ключе приходит от клиента: карта не должна расти без предела.
func TestLimiterBoundsMap(t *testing.T) {
	l := newLimiter(5, time.Hour)
	for i := range maxLimiterKeys * 2 {
		l.fail("login:" + strconv.Itoa(i))
	}
	if len(l.failures) > maxLimiterKeys {
		t.Errorf("ключей %d, ожидали не больше %d", len(l.failures), maxLimiterKeys)
	}
}

func TestLimiterPenaltyDoublesAndCaps(t *testing.T) {
	l := newLimiter(5, time.Minute)
	want := []time.Duration{0, time.Second, 2 * time.Second, 4 * time.Second,
		8 * time.Second, 16 * time.Second, 30 * time.Second, 30 * time.Second}
	for n, expect := range want {
		if got := l.penalty(n); got != expect {
			t.Errorf("penalty(%d) = %v, ожидали %v", n, got, expect)
		}
	}
}

func TestLimiterDelayGrowsWithFailures(t *testing.T) {
	l := newLimiter(5, time.Minute)
	l.delayUnit = 20 * time.Millisecond
	l.delayCap = time.Second

	if elapsed := timeDelay(t, l, "login:admin"); elapsed > 10*time.Millisecond {
		t.Errorf("без неудач ждали %v, ожидали ответ сразу", elapsed)
	}
	for range 3 {
		l.fail("login:admin")
	}
	// Третья подряд неудача — четыре единицы.
	if elapsed := timeDelay(t, l, "login:admin"); elapsed < 4*l.delayUnit {
		t.Errorf("после трёх неудач ждали %v, ожидали не меньше %v", elapsed, 4*l.delayUnit)
	}

	l.reset("login:admin")
	if elapsed := timeDelay(t, l, "login:admin"); elapsed > 10*time.Millisecond {
		t.Errorf("после reset ждали %v, ожидали ответ сразу", elapsed)
	}
}

func TestLimiterDelayStopsOnCancelledContext(t *testing.T) {
	l := newLimiter(5, time.Minute)
	l.delayUnit = time.Minute
	l.delayCap = time.Hour
	l.fail("login:admin")

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	start := time.Now()
	if !l.delay(ctx, "login:admin") {
		t.Fatal("delay отказал, ожидали разрешение")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("отменённый запрос ждал %v, ожидали немедленный возврат", elapsed)
	}
	if l.delayed != 0 {
		t.Errorf("счётчик задержанных %d, ожидали 0", l.delayed)
	}
}

// Сверх потолка запросы не встают в очередь, а сразу получают отказ.
func TestLimiterDelayShedsBeyondInFlightCap(t *testing.T) {
	l := newLimiter(5, time.Minute)
	l.delayUnit = time.Minute
	l.delayCap = time.Hour
	l.fail("login:admin")

	l.mu.Lock()
	l.delayed = maxDelayedInFlight
	l.mu.Unlock()

	start := time.Now()
	if l.delay(t.Context(), "login:admin") {
		t.Error("delay разрешил ждать сверх потолка")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("отказ занял %v, ожидали немедленный", elapsed)
	}
	if l.delayed != maxDelayedInFlight {
		t.Errorf("счётчик задержанных %d, ожидали %d", l.delayed, maxDelayedInFlight)
	}
}

func timeDelay(t *testing.T, l *limiter, key string) time.Duration {
	t.Helper()
	start := time.Now()
	if !l.delay(t.Context(), key) {
		t.Fatal("delay отказал, ожидали разрешение")
	}
	return time.Since(start)
}

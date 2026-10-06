package httpapi

import (
	"context"
	"sync"
	"time"
)

// maxLimiterKeys ограничивает счётчик по памяти: логин в ключе приходит от
// клиента, и без потолка перебор случайных логинов раздул бы карту.
const maxLimiterKeys = 4096

const (
	// defaultDelayUnit и defaultDelayCap задают прогрессию задержки ответа:
	// N-я подряд неудача по логину отвечает через 2^(N-1) единиц — 1, 2, 4, 8
	// секунды — и не дольше потолка.
	defaultDelayUnit = time.Second
	defaultDelayCap  = 30 * time.Second

	// maxDelayedInFlight ограничивает число запросов, одновременно висящих в
	// задержке: раз обработчик спит, всплеск неудач иначе занял бы горутину и
	// соединение на каждую попытку. Сверх потолка запросы не копятся в очереди,
	// а сразу получают обычный ответ о превышении лимита.
	maxDelayedInFlight = 64
)

// limiter считает неудачные попытки по ключу в скользящем окне. Хранит всё в
// памяти: сервис однопроцессный, переживать рестарт этим счётчикам не нужно.
//
// Ключей у попытки входа два, и работают они по-разному. По адресу клиента —
// жёсткая блокировка: перебор запирает только собственный адрес нападающего.
// По логину — только задержка ответа (delay): учётная запись в приложении одна
// и общая, так что блокировка по логину дала бы любому знающему логин запирать
// вход всем коллегам. Задержка тормозит перебор, но никого не оставляет за
// дверью — после ожидания пароль проверяется как обычно.
type limiter struct {
	mu       sync.Mutex
	failures map[string][]time.Time
	limit    int
	window   time.Duration

	// Шаг и потолок прогрессии — поля, а не константы, чтобы тесты не спали секундами.
	delayUnit time.Duration
	delayCap  time.Duration
	delayed   int
}

func newLimiter(limit int, window time.Duration) *limiter {
	return &limiter{
		failures:  make(map[string][]time.Time),
		limit:     limit,
		window:    window,
		delayUnit: defaultDelayUnit,
		delayCap:  defaultDelayCap,
	}
}

// penalty переводит число неудач в задержку: 2^(n-1) единиц, но не выше потолка.
// Удвоение в цикле, а не сдвигом, — цикл упирается в потолок и не переполняется.
func (l *limiter) penalty(n int) time.Duration {
	if n <= 0 {
		return 0
	}
	d := l.delayUnit
	for i := 1; i < n && d < l.delayCap; i++ {
		d *= 2
	}
	if d > l.delayCap {
		return l.delayCap
	}
	return d
}

// delay придерживает ответ на срок, зависящий только от числа свежих неудач по
// ключу, и возвращает false, если мест в задержке уже нет — тогда вызывающий
// отвечает как при превышении лимита, вместо того чтобы вставать в очередь.
//
// Ожидание прерывается вместе с запросом: ушедший клиент не держит горутину.
func (l *limiter) delay(ctx context.Context, key string) bool {
	l.mu.Lock()
	d := l.penalty(len(l.fresh(key)))
	if d <= 0 {
		l.mu.Unlock()
		return true
	}
	if l.delayed >= maxDelayedInFlight {
		l.mu.Unlock()
		return false
	}
	l.delayed++
	l.mu.Unlock()

	defer func() {
		l.mu.Lock()
		l.delayed--
		l.mu.Unlock()
	}()

	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
	}
	return true
}

// blocked сообщает, что хотя бы по одному из ключей исчерпан лимит попыток.
func (l *limiter) blocked(keys ...string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, key := range keys {
		if len(l.fresh(key)) >= l.limit {
			return true
		}
	}
	return false
}

// fail отмечает неудачную попытку по всем ключам.
func (l *limiter) fail(keys ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.evict()
	now := time.Now()
	for _, key := range keys {
		l.failures[key] = append(l.fresh(key), now)
	}
}

// reset снимает счётчики: удачный вход не должен оставлять хвост неудач.
func (l *limiter) reset(keys ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, key := range keys {
		delete(l.failures, key)
	}
}

// fresh возвращает попытки внутри окна, попутно выбрасывая просроченные.
// Вызывается под уже взятым замком.
func (l *limiter) fresh(key string) []time.Time {
	cutoff := time.Now().Add(-l.window)

	kept := l.failures[key][:0]
	for _, at := range l.failures[key] {
		if at.After(cutoff) {
			kept = append(kept, at)
		}
	}
	if len(kept) == 0 {
		delete(l.failures, key)
		return nil
	}
	l.failures[key] = kept
	return kept
}

// evict держит карту в границах: сначала чистит просроченное, а если и это не
// помогло — сбрасывает всё. Вызывается под уже взятым замком.
//
// ponytail: сброс целиком вместо вытеснения по возрасту — при переполнении
// нападающий разово теряет блокировку, но и получает её обратно за пять попыток.
func (l *limiter) evict() {
	if len(l.failures) < maxLimiterKeys {
		return
	}
	for key := range l.failures {
		l.fresh(key)
	}
	if len(l.failures) >= maxLimiterKeys {
		clear(l.failures)
	}
}

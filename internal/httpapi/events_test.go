package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"partnerops/internal/events"
)

func TestStreamSendsEvent(t *testing.T) {
	bus := events.NewBus()
	handler := NewEvents(bus)

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/events", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		handler.Stream(rec, req)
		close(done)
	}()

	// Даём обработчику подписаться до публикации.
	time.Sleep(50 * time.Millisecond)
	bus.Publish(events.Event{Kind: "snapshot.completed", Payload: map[string]any{"rows": 23}})
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("обработчик не завершился после отмены контекста")
	}

	body := rec.Body.String()
	if !strings.Contains(body, "event: snapshot.completed") {
		t.Errorf("в потоке нет имени события, тело: %q", body)
	}
	if !strings.Contains(body, `"rows":23`) {
		t.Errorf("в потоке нет полезной нагрузки, тело: %q", body)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Errorf("Content-Type = %q, ожидали text/event-stream", got)
	}
}

func TestStreamStopsOnDisconnect(t *testing.T) {
	bus := events.NewBus()
	handler := NewEvents(bus)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // клиент отключился сразу

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/events", nil).WithContext(ctx)
	done := make(chan struct{})
	go func() {
		handler.Stream(httptest.NewRecorder(), req)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("обработчик не завершился при отключённом клиенте")
	}
}

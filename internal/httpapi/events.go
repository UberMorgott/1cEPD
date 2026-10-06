package httpapi

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"partnerops/internal/events"
)

// keepAliveInterval не даёт обратному прокси разорвать простаивающее соединение.
const keepAliveInterval = 25 * time.Second

// Events отдаёт поток изменений состояния по Server-Sent Events.
type Events struct {
	bus *events.Bus
}

// NewEvents создаёт обработчик потока.
func NewEvents(bus *events.Bus) *Events {
	return &Events{bus: bus}
}

// Stream держит соединение и шлёт события, пока клиент не отключится.
func (e *Events) Stream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "no_streaming",
			"Сервер не умеет отдавать поток событий.")
		return
	}

	// Поток живёт часами, а у сервера общий WriteTimeout: снимаем дедлайн
	// именно здесь, чтобы он оставался у всех остальных обработчиков.
	if err := http.NewResponseController(w).SetWriteDeadline(time.Time{}); err != nil {
		slog.Warn("дедлайн записи не снят, поток событий может обрываться", "err", err)
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Отключает буферизацию в nginx, иначе события копятся и приходят пачкой.
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	stream, unsubscribe := e.bus.Subscribe()
	defer unsubscribe()

	keepAlive := time.NewTicker(keepAliveInterval)
	defer keepAlive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return

		case event, open := <-stream:
			if !open {
				return
			}
			payload, err := json.Marshal(event.Payload)
			if err != nil {
				slog.Error("не сериализовать событие", "kind", event.Kind, "err", err)
				continue
			}
			_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Kind, payload)
			flusher.Flush()

		case <-keepAlive.C:
			_, _ = fmt.Fprint(w, ": keep-alive\n\n")
			flusher.Flush()
		}
	}
}

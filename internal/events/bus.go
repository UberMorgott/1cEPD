// Package events рассылает изменения состояния подписчикам, а те отдают их фронту по SSE.
package events

import "sync"

// Event — одно изменение состояния.
type Event struct {
	Kind    string         `json:"kind"`
	Payload map[string]any `json:"payload,omitempty"`
}

// bufferSize подобран так, чтобы короткая пауза в чтении не приводила к потере событий.
const bufferSize = 16

// Bus рассылает события всем подписчикам. Медленный подписчик события теряет,
// но публикацию не тормозит: интерфейс всё равно перезапрашивает данные при подключении.
type Bus struct {
	mu          sync.Mutex
	subscribers map[int]chan Event
	nextID      int
}

// NewBus создаёт пустую шину.
func NewBus() *Bus {
	return &Bus{subscribers: make(map[int]chan Event)}
}

// Subscribe возвращает канал событий и функцию отписки. Отписка идемпотентна.
func (b *Bus) Subscribe() (<-chan Event, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()

	id := b.nextID
	b.nextID++
	ch := make(chan Event, bufferSize)
	b.subscribers[id] = ch

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			b.mu.Lock()
			defer b.mu.Unlock()
			if existing, ok := b.subscribers[id]; ok {
				delete(b.subscribers, id)
				close(existing)
			}
		})
	}
	return ch, unsubscribe
}

// Publish рассылает событие. Не блокируется, если подписчик не успевает читать.
func (b *Bus) Publish(event Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, ch := range b.subscribers {
		select {
		case ch <- event:
		default: // подписчик отстал, событие для него теряется
		}
	}
}

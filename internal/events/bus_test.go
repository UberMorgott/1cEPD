package events

import (
	"testing"
	"time"
)

func TestPublishReachesAllSubscribers(t *testing.T) {
	bus := NewBus()

	first, unsubFirst := bus.Subscribe()
	defer unsubFirst()
	second, unsubSecond := bus.Subscribe()
	defer unsubSecond()

	bus.Publish(Event{Kind: "snapshot.completed", Payload: map[string]any{"rows": 23}})

	for i, ch := range []<-chan Event{first, second} {
		select {
		case got := <-ch:
			if got.Kind != "snapshot.completed" {
				t.Errorf("подписчик %d получил %q", i, got.Kind)
			}
		case <-time.After(time.Second):
			t.Errorf("подписчик %d не получил событие", i)
		}
	}
}

func TestUnsubscribeStopsDelivery(t *testing.T) {
	bus := NewBus()
	ch, unsub := bus.Subscribe()
	unsub()

	bus.Publish(Event{Kind: "test"})

	select {
	case _, open := <-ch:
		if open {
			t.Error("канал отписавшегося подписчика получил событие")
		}
	case <-time.After(100 * time.Millisecond):
	}
}

func TestSlowSubscriberDoesNotBlockPublish(t *testing.T) {
	bus := NewBus()
	_, unsub := bus.Subscribe() // никто не читает
	defer unsub()

	done := make(chan struct{})
	go func() {
		for range 100 {
			bus.Publish(Event{Kind: "test"})
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Publish заблокировался на медленном подписчике")
	}
}

func TestUnsubscribeTwiceIsSafe(t *testing.T) {
	bus := NewBus()
	_, unsub := bus.Subscribe()
	unsub()
	unsub()
}

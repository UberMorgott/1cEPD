package main

import (
	"testing"
	"time"
)

func TestBrowseURL(t *testing.T) {
	cases := map[string]string{
		"127.0.0.1:8123": "http://127.0.0.1:8123",
		// Без хоста и на всех интерфейсах браузер открыть нельзя — нужна петля.
		":8080":       "http://127.0.0.1:8080",
		"0.0.0.0:80":  "http://127.0.0.1:80",
		"[::]:8080":   "http://127.0.0.1:8080",
		"[::1]:8080":  "http://[::1]:8080",
		"localhost:1": "http://localhost:1",
		// Мусор без порта отдаём как есть: пусть решает браузер.
		"пульт": "http://пульт",
	}

	for addr, want := range cases {
		if got := browseURL(addr); got != want {
			t.Errorf("browseURL(%q) = %q, ожидали %q", addr, got, want)
		}
	}
}

func TestPreviousMonth(t *testing.T) {
	cases := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "обычный месяц",
			now:  time.Date(2026, 9, 2, 17, 59, 0, 0, time.UTC),
			want: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "переход через год",
			now:  time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
			want: time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			// Наивное now.AddDate(0, -1, 0) здесь дало бы 3 марта: в феврале нет 31 числа.
			name: "31 марта не должно превратиться в март",
			now:  time.Date(2026, 3, 31, 23, 0, 0, 0, time.UTC),
			want: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "последний день года",
			now:  time.Date(2026, 12, 31, 12, 0, 0, 0, time.UTC),
			want: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := previousMonth(c.now); !got.Equal(c.want) {
				t.Errorf("previousMonth(%s) = %s, ожидали %s",
					c.now.Format(time.RFC3339), got.Format(time.RFC3339), c.want.Format(time.RFC3339))
			}
		})
	}
}

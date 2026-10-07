package mailer

import (
	"errors"
	"testing"
)

// Без сервера Send отказывает сразу, не подключаясь никуда (иначе пустой хост
// превратился бы в попытку соединения с ":465").
func TestSendWithoutHostFailsWithoutDialing(t *testing.T) {
	for _, host := range []string{"", "   "} {
		cfg := Config{Host: host, Port: 465, From: "robot@example.com", To: []string{"a@example.com"}}
		err := Send(t.Context(), cfg, "тема", "текст", "", nil)
		if !errors.Is(err, ErrNoServer) {
			t.Errorf("host %q: err = %v, ожидали ErrNoServer", host, err)
		}
	}
}

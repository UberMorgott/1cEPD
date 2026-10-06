package partner

import (
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

func TestProgramsByLoginParsesSupportConditions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.URL.Path != "/client-program-access/search/login" || string(body) != `{"login":"buh@romashka.ru"}` {
			t.Errorf("запрос %s %s", r.URL.Path, body)
		}
		// Регномер приходит то числом, то строкой: оба варианта должны разбираться.
		_, _ = w.Write([]byte(`[
		  {"regNumber":200001912345,"program":{"name":"Бухгалтерия предприятия"},"hasAccess":true,
		   "missingSupportConditions":[]},
		  {"regNumber":"200001967890","program":{"name":"Технологическая платформа"},"hasAccess":false,
		   "missingSupportConditions":[{"name":"Договор 1С:ИТС"},{"name":" "}]}
		]`))
	}))
	defer srv.Close()

	list, err := New(srv.URL, "u", "p").ProgramsByLogin(t.Context(), " buh@romashka.ru ")
	if err != nil {
		t.Fatalf("ProgramsByLogin: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("строк %d, ожидали 2", len(list))
	}
	if list[0].RegNumber != "200001912345" || list[0].Program != "Бухгалтерия предприятия" ||
		!list[0].HasAccess || len(list[0].MissingConditions) != 0 {
		t.Errorf("первая строка: %+v", list[0])
	}
	if list[1].RegNumber != "200001967890" || list[1].HasAccess ||
		!slices.Equal(list[1].MissingConditions, []string{"Договор 1С:ИТС"}) {
		t.Errorf("вторая строка: %+v", list[1])
	}
}

func TestProgramsByRegNumberRejectsNonNumber(t *testing.T) {
	if _, err := New("http://127.0.0.1:1", "u", "p").ProgramsByRegNumber(t.Context(), "12a"); err == nil {
		t.Fatal("нечисловой регномер должен отвергаться до запроса")
	}
}

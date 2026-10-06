package partner

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Фикстуры повторяют форму живых ответов; коды, номера и реквизиты вымышлены.
const (
	subscribersPage0 = `{"subscribers":[
	 {"code":"CL-1000001","name":"Абонент Альфа","subjects":["EDO"],"regNumbers":[800000001,800000002],
	  "organizations":[{"name":"ООО \"Альфа\"","inn":"7700000001","kpp":"770001001"}]}],
	 "size":1,"page":0,"total":2}`
	subscribersPage1 = `{"subscribers":[
	 {"code":"FR-FR-2000002","name":"Абонент Бета","subjects":["EDO","FRESH_PURCHASE"],"regNumbers":null,
	  "organizations":[{"name":"ИП Бета","inn":"770000000002","kpp":""}]}],
	 "size":1,"page":1,"total":2}`
)

func TestSubscribersReadsAllPages(t *testing.T) {
	var pages []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		pages = append(pages, string(body))
		if r.URL.Path != "/rest/public/subscriber" {
			t.Errorf("путь %s", r.URL.Path)
		}
		if string(body) == `{"page":0,"size":300}` {
			_, _ = w.Write([]byte(subscribersPage0))
			return
		}
		_, _ = w.Write([]byte(subscribersPage1))
	}))
	defer srv.Close()

	list, err := New(srv.URL, "u", "p").Subscribers(t.Context())
	if err != nil {
		t.Fatalf("Subscribers: %v", err)
	}
	if len(pages) != 2 || pages[1] != `{"page":1,"size":300}` {
		t.Fatalf("запросы страниц: %v", pages)
	}
	if len(list) != 2 {
		t.Fatalf("абонентов %d, ожидали 2", len(list))
	}
	first := list[0]
	if first.Code != "CL-1000001" || len(first.RegNumbers) != 2 || first.RegNumbers[0] != "800000001" ||
		len(first.Organizations) != 1 || first.Organizations[0].KPP != "770001001" {
		t.Errorf("первый абонент: %+v", first)
	}
	if list[1].RegNumbers != nil || list[1].Organizations[0].INN != "770000000002" {
		t.Errorf("второй абонент: %+v", list[1])
	}
}

func TestNomenclatureSendsBareArray(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.URL.Path != "/nomenclature/getByRegNumbers" || string(body) != `[800000001,800000009]` {
			t.Errorf("запрос %s %s", r.URL.Path, body)
		}
		_, _ = w.Write([]byte(`[{"regNum":800000001,"serialNumber":"4601546000001","name":"Бухгалтерия предприятия"}]`))
	}))
	defer srv.Close()

	list, err := New(srv.URL, "u", "p").NomenclatureByRegNumbers(t.Context(), []string{"800000001", " 800000009 "})
	if err != nil {
		t.Fatalf("NomenclatureByRegNumbers: %v", err)
	}
	if len(list) != 1 || list[0].RegNumber != "800000001" || list[0].Name != "Бухгалтерия предприятия" {
		t.Errorf("номенклатура: %+v", list)
	}
	if _, err := New(srv.URL, "u", "p").NomenclatureByRegNumbers(t.Context(), []string{"abc"}); err == nil {
		t.Error("нечисловой регномер должен давать ошибку до запроса")
	}
}

func TestCheckITSByRegNumbersKeepsRegNumber(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.URL.Path != "/rest/public/subscription/checkItsByRegNum" || string(body) != `{"regNumberList":[800000001]}` {
			t.Errorf("запрос %s %s", r.URL.Path, body)
		}
		_, _ = w.Write([]byte(`[{"status":"Success","code":1,"description":"","regNumber":"800000001",
		  "itsContractInfo":[{"startDate":"2026-03-31T21:00:00Z","endDate":"2027-03-31T20:59:59Z",
		  "itsContractType":{"name":"ИТС ПРОФ","nameForUser":"1С:Комплект поддержки ПРОФ","publicSubscriptionTypeNumber":130}}]}]`))
	}))
	defer srv.Close()

	checks, err := New(srv.URL, "u", "p").CheckITSByRegNumbers(t.Context(), []string{"800000001"})
	if err != nil {
		t.Fatalf("CheckITSByRegNumbers: %v", err)
	}
	if len(checks) != 1 || checks[0].RegNumber != "800000001" || len(checks[0].Contracts) != 1 ||
		*checks[0].Contracts[0].TypeNumber != 130 {
		t.Errorf("проверка: %+v", checks)
	}
}

func TestUserExists(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if r.URL.Path != "/users" || body["login"] != "client" || body["email"] != "client@example.ru" {
			t.Errorf("запрос %s %v", r.URL.Path, body)
		}
		_, _ = w.Write([]byte(`{"found":true}`))
	}))
	defer srv.Close()

	found, err := New(srv.URL, "u", "p").UserExists(t.Context(), " client ", "client@example.ru")
	if err != nil || !found {
		t.Errorf("UserExists = %v, %v", found, err)
	}
}

func TestIsLoginNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"status":"LoginNotFound","description":"Логин не найден","login":"x"}`))
	}))
	defer srv.Close()

	_, err := New(srv.URL, "u", "p").ProgramsByLogin(t.Context(), "x")
	if !IsLoginNotFound(err) {
		t.Errorf("ожидали LoginNotFound, получили %v", err)
	}
	if IsLoginNotFound(&APIError{StatusCode: http.StatusNotFound, Message: "Not Found"}) {
		t.Error("обычный 404 — не LoginNotFound")
	}
}

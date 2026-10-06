package partner

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// itsFixture повторяет форму живого ответа checkItsBySubscriberCode
// (коды абонентов и описания заменены).
const itsFixture = `[
 {"status":"Success","code":1,"description":"Договор 1С:ИТС оформлен.","subscriberCode":"CL-1000001",
  "itsContractInfo":[
   {"description":"Договор оформлен другим партнером с 01-04-2026 по 30-09-2026. ИТС уровня Базовый",
    "startDate":"2026-03-31T21:00:00Z","endDate":"2026-09-30T20:59:59Z",
    "itsContractType":{"uin":"u-160","name":"1С:ИТС уровня Базовый",
     "nameForUser":"1С:Комплект поддержки Базовый","publicSubscriptionTypeNumber":160}},
   {"description":"","startDate":"2026-09-30T21:00:00Z","endDate":"2027-03-31T20:59:59Z",
    "itsContractType":{"uin":"u-160","name":"1С:ИТС уровня Базовый",
     "nameForUser":"1С:Комплект поддержки Базовый","publicSubscriptionTypeNumber":null}}]},
 {"status":"IsNotClient","code":109,"description":"Для абонента проверка не предусмотрена.",
  "subscriberCode":"FR-FR-2000002","itsContractInfo":[]}
]`

func TestCheckITSBySubscriberCodesParsesContracts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.URL.Path != "/rest/public/subscription/checkItsBySubscriberCode" ||
			string(body) != `{"subscriberCodeList":["CL-1000001","FR-FR-2000002"]}` {
			t.Errorf("запрос %s %s", r.URL.Path, body)
		}
		_, _ = w.Write([]byte(itsFixture))
	}))
	defer srv.Close()

	checks, err := New(srv.URL, "u", "p").CheckITSBySubscriberCodes(t.Context(),
		[]string{"CL-1000001", "FR-FR-2000002"})
	if err != nil {
		t.Fatalf("CheckITSBySubscriberCodes: %v", err)
	}
	if len(checks) != 2 {
		t.Fatalf("ответов %d, ожидали 2", len(checks))
	}
	first := checks[0]
	if first.SubscriberCode != "CL-1000001" || first.Code != ITSStatusSuccess || len(first.Contracts) != 2 {
		t.Fatalf("первый ответ: %+v", first)
	}
	c := first.Contracts[0]
	if c.TypeNumber == nil || *c.TypeNumber != 160 || c.TypeNameForUser != "1С:Комплект поддержки Базовый" ||
		!c.End.Equal(time.Date(2026, 9, 30, 20, 59, 59, 0, time.UTC)) {
		t.Errorf("договор: %+v", c)
	}
	// null в номере вида — не ноль, а «неизвестно».
	if first.Contracts[1].TypeNumber != nil {
		t.Errorf("null разобран как %d", *first.Contracts[1].TypeNumber)
	}
	if checks[1].Code != 109 || len(checks[1].Contracts) != 0 {
		t.Errorf("второй ответ: %+v", checks[1])
	}
}

func TestCheckITSBySubscriberCodesSplitsBatches(t *testing.T) {
	var sizes []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			List []string `json:"subscriberCodeList"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		sizes = append(sizes, len(body.List))
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	codes := make([]string, 205)
	for i := range codes {
		codes[i] = fmt.Sprintf("CL-%d", i)
	}
	if _, err := New(srv.URL, "u", "p").CheckITSBySubscriberCodes(t.Context(), codes); err != nil {
		t.Fatalf("CheckITSBySubscriberCodes: %v", err)
	}
	if fmt.Sprint(sizes) != "[100 100 5]" {
		t.Errorf("пачки %v, ожидали [100 100 5]", sizes)
	}
}

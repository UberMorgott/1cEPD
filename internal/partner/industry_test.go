package partner

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// industryFixture — формы живых ответов checkIndustryBySubscriberCode
// (106 и 109 с null) плюс 107 из docs/API.md §6; коды и имена заменены.
const industryFixture = `[
 {"status":"NotEmptyList","code":107,"description":"Есть конфигурация, которой нужен ИТС Отраслевой",
  "subscriberCode":"CL-1000001","programInfoList":[
   {"uin":"p-1","name":"1С:Учет в управляющих компаниях ЖКХ","industrySubscriptionInfoList":null},
   {"uin":"p-2","name":"1С:Медицина","industrySubscriptionInfoList":[
    {"nomenclatureName":"ИТС Отраслевой","serialNumber":"123","industrySubscriptionTypeInfo":{"description":"Отраслевой","uin":"t"},
     "beginDate":"2026-01-31T21:00:00Z","endDate":"2027-01-31T20:59:59Z"}]}]},
 {"status":"EmptyList","code":106,"description":"Нет конфигураций","subscriberCode":"CL-1000002","programInfoList":null},
 {"status":"IsNotCLIENT","code":109,"description":"Не предусмотрена","subscriberCode":"FR-FR-3","programInfoList":null}
]`

func TestCheckIndustryFindsMissingSubscription(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.URL.Path != "/rest/public/industry/checkIndustryBySubscriberCode" ||
			string(body) != `{"subscriberCodeList":["CL-1000001","CL-1000002","FR-FR-3"]}` {
			t.Errorf("запрос %s %s", r.URL.Path, body)
		}
		_, _ = w.Write([]byte(industryFixture))
	}))
	defer srv.Close()

	checks, err := New(srv.URL, "u", "p").CheckIndustryBySubscriberCodes(t.Context(),
		[]string{"CL-1000001", "CL-1000002", "FR-FR-3"})
	if err != nil {
		t.Fatalf("CheckIndustryBySubscriberCodes: %v", err)
	}
	if len(checks) != 3 {
		t.Fatalf("ответов %d", len(checks))
	}
	missing := checks[0].MissingIndustry()
	if len(missing) != 1 || missing[0].Name != "1С:Учет в управляющих компаниях ЖКХ" {
		t.Errorf("не оформлен: %+v", missing)
	}
	sub := checks[0].Programs[1].Subscriptions
	if len(sub) != 1 || !sub[0].End.Equal(time.Date(2027, 1, 31, 20, 59, 59, 0, time.UTC)) ||
		sub[0].TypeDescription != "Отраслевой" {
		t.Errorf("оформленный: %+v", sub)
	}
	for _, check := range checks[1:] {
		if check.MissingIndustry() != nil || len(check.Programs) != 0 {
			t.Errorf("%s: нужды нет, а найдено %+v", check.SubscriberCode, check)
		}
	}
}

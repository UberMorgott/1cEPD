package partner

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestParseOptionReportFixture(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/option-billing-report.json")
	if err != nil {
		t.Fatalf("фикстура: %v", err)
	}
	report, err := ParseOptionReport(raw)
	if err != nil {
		t.Fatalf("ParseOptionReport: %v", err)
	}
	if report.State != OptionReportOK || len(report.Entries) != 7 || report.Completed.IsZero() {
		t.Fatalf("отчёт: state %q, абонентов %d", report.State, len(report.Entries))
	}
	first := report.Entries[0]
	if first.SubscriberCode != "FR-FR-631227" || len(first.Tariffs) != 1 {
		t.Fatalf("первый абонент %+v", first)
	}
	tariff := first.Tariffs[0]
	// null в виде договора — nil, а не 0.
	if tariff.TypeNumber != nil {
		t.Errorf("publicSubscriptionTypeNumber null разобран как %d", *tariff.TypeNumber)
	}
	if tariff.End != time.Date(2027, 8, 25, 20, 59, 59, 0, time.UTC) || tariff.OrgINN == "" {
		t.Errorf("тариф %+v", tariff)
	}
	option := tariff.Options[0]
	if !option.Quantitative || option.MaxVolume == nil || *option.MaxVolume != 1 ||
		option.UsedVolume == nil || *option.UsedVolume != 1 {
		t.Errorf("опция %+v", option)
	}

	var numbered bool
	for _, e := range report.Entries {
		for _, tr := range e.Tariffs {
			if tr.TypeNumber != nil && *tr.TypeNumber == 161 {
				numbered = true
			}
		}
	}
	if !numbered {
		t.Error("вид договора 161 потерялся")
	}
}

func TestParseOptionReportProcessingHasNoReport(t *testing.T) {
	report, err := ParseOptionReport([]byte(`{"reportUeid":"x","state":"PROCESSING"}`))
	if err != nil || report.State != OptionReportProcessing || report.Entries != nil {
		t.Fatalf("PROCESSING: %+v %v", report, err)
	}
	report, err = ParseOptionReport([]byte(`{"state":"OK","report":{"entries":[
		{"subscriberCode":"CL-1","tariffs":[{"name":"Т","options":[{"name":"качество","quantitative":false}]}]},
		{"subscriberCode":"CL-2","tariffs":[]}]}}`))
	if err != nil || len(report.Entries) != 2 || report.Entries[1].Tariffs != nil {
		t.Fatalf("пустые списки: %+v %v", report, err)
	}
	if o := report.Entries[0].Tariffs[0].Options[0]; o.MaxVolume != nil || o.UsedVolume != nil {
		t.Errorf("у качественной опции объёмы %+v", o)
	}
}

func TestOptionBillingReportPollsUntilOK(t *testing.T) {
	var polls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/rest/public/option/billing-report":
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), `"type":"SIGN"`) {
				t.Errorf("тело %s", body)
			}
			_, _ = w.Write([]byte(`{"reportUeid":"r-1"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/rest/public/option/billing-report/r-1":
			if atomic.AddInt32(&polls, 1) < 3 {
				_, _ = w.Write([]byte(`{"reportUeid":"r-1","state":"PROCESSING"}`))
				return
			}
			_, _ = w.Write([]byte(`{"reportUeid":"r-1","state":"OK","report":{"entries":[{"subscriberCode":"CL-1","tariffs":[]}]}}`))
		default:
			t.Errorf("неожиданный запрос %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	c := New(srv.URL, "login", "secret")
	c.pollInterval = time.Millisecond
	report, err := c.OptionBillingReport(t.Context(), "SIGN")
	if err != nil || len(report.Entries) != 1 || atomic.LoadInt32(&polls) != 3 {
		t.Fatalf("отчёт %+v, опросов %d, ошибка %v", report, polls, err)
	}
}

func TestOptionBillingReportErrors(t *testing.T) {
	cases := map[string]struct {
		post, get string
		status    int
		check     func(error) bool
	}{
		"нет биллинга": {post: `{"error":"BILLING_DOES_NOT_EXIST","message":"нет"}`, status: http.StatusBadRequest,
			check: IsNoBilling},
		"лимит": {post: `{"error":"MAX_TASKS_PER_HOUR_LIMIT_REACHED","message":"лимит"}`, status: http.StatusBadRequest,
			check: IsRateLimited},
		"ERROR": {post: `{"reportUeid":"r-2"}`, status: http.StatusOK, get: `{"reportUeid":"r-2","state":"ERROR"}`,
			check: func(err error) bool { return err != nil && !IsRateLimited(err) && !IsNoBilling(err) }},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					w.WriteHeader(tc.status)
					_, _ = w.Write([]byte(tc.post))
					return
				}
				_, _ = w.Write([]byte(tc.get))
			}))
			defer srv.Close()
			c := New(srv.URL, "login", "secret")
			c.pollInterval = time.Millisecond
			if _, err := c.OptionBillingReport(t.Context(), "REPORTING"); !tc.check(err) {
				t.Errorf("ошибка %v", err)
			}
		})
	}
}

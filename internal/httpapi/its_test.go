package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"partnerops/internal/partner"
	"partnerops/internal/store"
)

type stubITSChecks []store.ITSCheck

func (s stubITSChecks) All(ctx context.Context) ([]store.ITSCheck, error) { return s, nil }

type ownersRegistry struct{ stubRegistry }

func (ownersRegistry) All(ctx context.Context) ([]store.IdentifierRecord, error) {
	return []store.IdentifierRecord{
		{EDOID: "2AE-1", OwnerCode: "CL-1", INN: "7700000001", KPP: "770001001", ClientName: "ООО Альфа",
			ITSTariffs: "1С-ЭДО. ЭПД-600(0): подп. ИТС №: 1 | 1С:ИТС уровня ПРОФ(100): подп. ИТС №: 2"},
		{EDOID: "2AE-2", OwnerCode: "CL-1", INN: "7700000001", KPP: "770001001", ClientName: "ООО Альфа"},
	}, nil
}

func TestITSContractsListsExpiringWithRenewal(t *testing.T) {
	number := 160
	end := time.Now().UTC().AddDate(0, 0, 5)
	checks := stubITSChecks{{SubscriberCode: "CL-1", Code: 1,
		Contracts: []partner.ITSContract{{TypeName: "Базовый", TypeNumber: &number, End: end}}}}
	h := NewRegistry(&ownersRegistry{}, nil).WithITS(checks, nil)

	rec := httptest.NewRecorder()
	h.ITSContracts(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/its/contracts?days=10", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("код %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Expiring []struct {
			DaysLeft int `json:"daysLeft"`
			Clients  []struct {
				INN string `json:"inn"`
			} `json:"clients"`
			Renewal renewalDraft `json:"renewal"`
		} `json:"expiring"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("ответ: %v", err)
	}
	if len(body.Expiring) != 1 {
		t.Fatalf("напоминаний %d, ожидали 1: %s", len(body.Expiring), rec.Body.String())
	}
	e := body.Expiring[0]
	// Два идентификатора одной организации — одна организация в списке.
	if len(e.Clients) != 1 || e.Clients[0].INN != "7700000001" || e.DaysLeft != 4 && e.DaysLeft != 5 {
		t.Errorf("напоминание %+v", e)
	}
	// Вид 160 — не код ЭПД, поэтому тариф берётся из биллинга: ЭПД-600 = 2080.
	if e.Renewal.TariffCode != "2080" || e.Renewal.StartDate != end.Add(time.Second).In(moscow).Format("02.01.06") {
		t.Errorf("предзаполнение %+v", e.Renewal)
	}
}

type stubIndustry []store.IndustryCheck

func (s stubIndustry) All(ctx context.Context) ([]store.IndustryCheck, error) { return s, nil }

func TestITSContractsFlagsMissingIndustry(t *testing.T) {
	checks := stubITSChecks{{SubscriberCode: "CL-1", Code: 1}}
	industry := stubIndustry{{SubscriberCode: "CL-1", Code: partner.IndustryStatusNeeded,
		Programs: []partner.IndustryProgram{{Name: "1С:ЖКХ"}}}}
	h := NewRegistry(&ownersRegistry{}, nil).WithITS(checks, nil).WithIndustry(industry)

	rec := httptest.NewRecorder()
	h.ITSContracts(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/its/contracts", nil))
	var body struct {
		Subscribers []struct {
			Industry *industryItem `json:"industry"`
		} `json:"subscribers"`
		IndustryMissing []industryMissingItem `json:"industryMissing"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("ответ %d: %v", rec.Code, err)
	}
	if len(body.IndustryMissing) != 1 || body.IndustryMissing[0].Programs[0] != "1С:ЖКХ" ||
		len(body.IndustryMissing[0].Clients) != 1 {
		t.Errorf("нужен Отраслевой: %+v", body.IndustryMissing)
	}
	if len(body.Subscribers) != 1 || body.Subscribers[0].Industry == nil ||
		body.Subscribers[0].Industry.StatusCode != 107 {
		t.Errorf("абонент: %s", rec.Body.String())
	}
}

func TestITSContractsRejectsBadWindow(t *testing.T) {
	h := NewRegistry(&ownersRegistry{}, nil).WithITS(stubITSChecks{}, nil)
	rec := httptest.NewRecorder()
	h.ITSContracts(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/its/contracts?days=0", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("код %d, ожидали 400", rec.Code)
	}
}

func TestRenewalStartsNextMoscowDay(t *testing.T) {
	// 1С пишет конец договора как 20:59:59Z — это 23:59:59 по Москве.
	c := partner.ITSContract{End: time.Date(2026, 9, 30, 20, 59, 59, 0, time.UTC)}
	if got := renewal(c, nil); got.StartDate != "01.10.26" || got.TariffCode != "" {
		t.Errorf("предзаполнение %+v", got)
	}
	epd := 2093
	c.TypeNumber = &epd
	if got := renewal(c, nil); got.TariffCode != "2093" {
		t.Errorf("вид договора — код ЭПД, а тариф %q", got.TariffCode)
	}
}

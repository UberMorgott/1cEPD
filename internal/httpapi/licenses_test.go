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

type stubOptionReports []store.OptionReport

func (s stubOptionReports) All(ctx context.Context) ([]store.OptionReport, error) { return s, nil }

func TestLicensesListsExpiringAndLow(t *testing.T) {
	now := time.Now().UTC()
	five, used := 5.0, 5.0
	one := 1.0
	tariff := partner.OptionTariff{Name: "ИТСааС ПРОФ", OrgINN: "7700000001", End: now.AddDate(0, 0, 5),
		Options: []partner.Option{{Name: "Подписи", Quantitative: true, MaxVolume: &five, UsedVolume: &used}}}
	reporting := tariff
	reporting.Options = []partner.Option{{Name: "Лицензия", Quantitative: true, MaxVolume: &one, UsedVolume: &one}}
	reports := stubOptionReports{
		{Type: "REPORTING", State: store.OptionStateOK, FetchedAt: now,
			Entries: []partner.OptionEntry{{SubscriberCode: "CL-1", Tariffs: []partner.OptionTariff{reporting}}}},
		{Type: "SIGN", State: store.OptionStateOK, FetchedAt: now,
			Entries: []partner.OptionEntry{{SubscriberCode: "CL-1", Tariffs: []partner.OptionTariff{tariff}}}},
		{Type: "ESS", State: store.OptionStateNone, FetchedAt: now},
	}
	h := NewRegistry(&ownersRegistry{}, nil).WithLicenses(reports)

	rec := httptest.NewRecorder()
	h.Licenses(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/licenses?days=10", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("код %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Reports     []licenseReportItem     `json:"reports"`
		Subscribers []licenseSubscriberItem `json:"subscribers"`
		Expiring    []licenseExpiringItem   `json:"expiring"`
		Low         []licenseLowItem        `json:"low"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("ответ: %v", err)
	}
	if len(body.Reports) != 3 || len(body.Subscribers) != 1 || len(body.Subscribers[0].Tariffs) != 1 {
		t.Fatalf("отчёты и абоненты: %s", rec.Body.String())
	}
	if services := body.Subscribers[0].Tariffs[0].Services; len(services) != 2 || services[0] != "REPORTING" {
		t.Errorf("сервисы тарифа %v", services)
	}
	if len(body.Expiring) != 1 || body.Expiring[0].DaysLeft < 4 || len(body.Expiring[0].Clients) != 1 {
		t.Errorf("кончаются: %+v", body.Expiring)
	}
	if len(body.Low) != 1 || body.Low[0].Option.Name != "Подписи" || body.Low[0].Option.Remaining == nil ||
		*body.Low[0].Option.Remaining != 0 {
		t.Errorf("мало остатка: %+v", body.Low)
	}
}

func TestLicensesNotConfigured(t *testing.T) {
	h := NewRegistry(&ownersRegistry{}, nil)
	rec := httptest.NewRecorder()
	h.Licenses(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/licenses", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("код %d", rec.Code)
	}
}

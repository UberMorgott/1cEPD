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

type stubUsage struct {
	ok   bool
	rows []partner.EDOTrafficRow
}

func (s stubUsage) Run(ctx context.Context) (store.EPDUsageRun, bool, error) {
	return store.EPDUsageRun{PeriodFrom: "2025-09", PeriodTo: "2026-08", FetchedAt: time.Now()}, s.ok, nil
}

func (s stubUsage) Rows(ctx context.Context) ([]partner.EDOTrafficRow, error) { return s.rows, nil }

func TestEPDAdviceSuggestsTariff(t *testing.T) {
	usage := stubUsage{ok: true, rows: []partner.EDOTrafficRow{
		{INN: "7700000001", KPP: "770001001", Subscriber: "CL-1", EPDOut: 1000},
	}}
	// В реестре у организации ЭПД-600 (ownersRegistry из its_test.go).
	h := NewRegistry(&ownersRegistry{}, nil).WithEPDUsage(usage)

	rec := httptest.NewRecorder()
	h.EPDAdvice(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/its/epd-advice", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("код %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		PeriodTo string          `json:"periodTo"`
		Clients  []epdAdviceItem `json:"clients"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("ответ: %v", err)
	}
	if body.PeriodTo != "2026-08" || len(body.Clients) != 1 {
		t.Fatalf("ответ %s", rec.Body.String())
	}
	c := body.Clients[0]
	if c.Best == nil || c.Best.Code != "2081" || len(c.Current) != 1 || c.Current[0].Code != "2080" ||
		c.Savings != "1400,00" || c.Optimal {
		t.Errorf("подсказка %+v", c)
	}
}

func TestEPDAdviceEmptyBeforeFirstReport(t *testing.T) {
	h := NewRegistry(&ownersRegistry{}, nil).WithEPDUsage(stubUsage{})
	rec := httptest.NewRecorder()
	h.EPDAdvice(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/its/epd-advice", nil))
	if rec.Code != http.StatusOK || !json.Valid(rec.Body.Bytes()) {
		t.Fatalf("код %d: %s", rec.Code, rec.Body.String())
	}
}

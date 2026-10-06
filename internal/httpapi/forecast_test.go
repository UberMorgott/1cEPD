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

type limitRegistry struct{ stubRegistry }

func (s *limitRegistry) All(ctx context.Context) ([]store.IdentifierRecord, error) {
	limit := int64(10)
	return []store.IdentifierRecord{{EDOID: "2AE-A", ClientName: "ООО Тест", Limit: &limit}}, nil
}

func anomalyKinds(t *testing.T, h *Registry, query string) []string {
	t.Helper()
	rec := httptest.NewRecorder()
	h.Anomalies(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/anomalies"+query, nil))
	var body struct {
		Anomalies []struct {
			ID          int64  `json:"id"`
			Kind        string `json:"kind"`
			Fingerprint string `json:"fingerprint"`
		} `json:"anomalies"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("ответ %d: %v", rec.Code, err)
	}
	var kinds []string
	for _, a := range body.Anomalies {
		kinds = append(kinds, a.Kind)
	}
	return kinds
}

// Прогноз перерасхода живёт в биллинге: находкой он не становится, даже когда
// идентификатор уже вышел за лимит.
func TestForecastStaysInBillingNotAnomalies(t *testing.T) {
	month := stubUsage{ok: true, rows: []partner.EDOTrafficRow{{EDOID: "2AE-A", InvoicesOut: 25}}}
	reg := &limitRegistry{stubRegistry{events: []store.AnomalyEvent{{ID: 1, EDOID: "2AE-B", Kind: "orphan_idle"}}}}
	current := currentMonth{stubUsage: month}
	h := NewRegistry(reg, nil).WithForecast(current)
	if kinds := anomalyKinds(t, h, "?all=1"); len(kinds) != 1 || kinds[0] != "orphan_idle" {
		t.Fatalf("находки %v", kinds)
	}

	rec := httptest.NewRecorder()
	h.BillingForecast(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/billing/forecast", nil))
	var forecast struct {
		Clients []struct {
			EDOID    string `json:"edoId"`
			Exceeded bool   `json:"exceeded"`
		} `json:"clients"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &forecast); err != nil || len(forecast.Clients) != 1 ||
		!forecast.Clients[0].Exceeded {
		t.Errorf("прогноз %s: %v", rec.Body.String(), err)
	}
}

// currentMonth выдаёт прогон за текущий месяц.
type currentMonth struct{ stubUsage }

func (c currentMonth) Run(ctx context.Context) (store.EPDUsageRun, bool, error) {
	now := time.Now().UTC()
	return store.EPDUsageRun{PeriodFrom: now.Format("2006-01"), PeriodTo: now.Format("2006-01"), FetchedAt: now}, true, nil
}

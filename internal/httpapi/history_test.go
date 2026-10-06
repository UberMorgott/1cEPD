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

type monthSnapshots map[string][]partner.EDOBillingRow

func (m monthSnapshots) Latest(ctx context.Context, period string) (store.Snapshot, []partner.EDOBillingRow, error) {
	rows, ok := m[period]
	if !ok {
		return store.Snapshot{}, nil, store.ErrNoSnapshot
	}
	return store.Snapshot{Period: period}, rows, nil
}

type stubAttempts map[string]store.BackfillAttempt

func (s stubAttempts) Attempts(ctx context.Context) (map[string]store.BackfillAttempt, error) {
	return s, nil
}

func TestBillingHistoryReturnsTwelveMonths(t *testing.T) {
	last := previousPeriod(time.Now())
	snapshots := monthSnapshots{last: {{EDOID: "2AE-A", Packets: 7}}}
	h := NewRegistry(&stubRegistry{}, snapshots).WithHistory(stubAttempts{})

	rec := httptest.NewRecorder()
	h.BillingHistory(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/billing/history", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("код %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Months []struct {
			Period  string `json:"period"`
			Status  string `json:"status"`
			Packets int64  `json:"packets"`
		} `json:"months"`
		Clients []struct {
			EDOID   string   `json:"edoId"`
			Packets []*int64 `json:"packets"`
		} `json:"clients"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("ответ: %v", err)
	}
	if len(body.Months) != 12 || body.Months[11].Period != last || body.Months[11].Status != "ok" ||
		body.Months[11].Packets != 7 || body.Months[0].Status != "pending" {
		t.Errorf("месяцы %+v", body.Months)
	}
	if len(body.Clients) != 1 || len(body.Clients[0].Packets) != 12 || body.Clients[0].Packets[0] != nil {
		t.Errorf("клиенты %+v", body.Clients)
	}
}

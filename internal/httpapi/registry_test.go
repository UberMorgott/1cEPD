package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"partnerops/internal/store"
)

type stubRegistry struct {
	events []store.AnomalyEvent
	acked  []string
}

func (s *stubRegistry) Events(ctx context.Context, includeAcknowledged bool) ([]store.AnomalyEvent, error) {
	return s.events, nil
}

func (s *stubRegistry) All(ctx context.Context) ([]store.IdentifierRecord, error) {
	return []store.IdentifierRecord{{EDOID: "2AE-A", ClientName: "ООО Тест"}}, nil
}

func (s *stubRegistry) Acknowledge(ctx context.Context, edoID, fingerprint, reason, author string, at time.Time) error {
	s.acked = append(s.acked, edoID+"/"+fingerprint+"/"+author)
	return nil
}

func TestAnomaliesReturnsList(t *testing.T) {
	reg := &stubRegistry{events: []store.AnomalyEvent{
		{ID: 1, EDOID: "2AE-A", Kind: "orphan_with_traffic", Confidence: "high"},
	}}
	h := NewRegistry(reg, nil)

	rec := httptest.NewRecorder()
	h.Anomalies(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/anomalies", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("код %d, ожидали 200", rec.Code)
	}
	var body struct {
		Anomalies []struct {
			EDOID string `json:"edoId"`
			Kind  string `json:"kind"`
		} `json:"anomalies"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("не разобрать ответ: %v", err)
	}
	if len(body.Anomalies) != 1 || body.Anomalies[0].EDOID != "2AE-A" {
		t.Errorf("в ответе %+v", body.Anomalies)
	}
}

func TestAcknowledgeMarksEvent(t *testing.T) {
	reg := &stubRegistry{}
	h := NewRegistry(reg, nil)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/anomalies/ack",
		strings.NewReader(`{"edoId":"2AE-A","fingerprint":"fp-1","reason":"законно"}`))
	req = req.WithContext(context.WithValue(req.Context(), sessionLoginKey{}, "buh"))
	rec := httptest.NewRecorder()
	h.Acknowledge(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("код %d, ожидали 200, тело %s", rec.Code, rec.Body.String())
	}
	// Автор пометки — вошедший пользователь.
	if len(reg.acked) != 1 || reg.acked[0] != "2AE-A/fp-1/buh" {
		t.Errorf("подтверждено %v", reg.acked)
	}
}

func TestAcknowledgeRejectsEmptyFingerprint(t *testing.T) {
	// Без отпечатка подтверждение погасило бы находку навсегда, включая будущие поломки.
	h := NewRegistry(&stubRegistry{}, nil)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/anomalies/ack",
		strings.NewReader(`{"edoId":"2AE-A","fingerprint":""}`))
	rec := httptest.NewRecorder()
	h.Acknowledge(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("код %d, ожидали 400", rec.Code)
	}
}

func TestIdentifiersReturnsRegistry(t *testing.T) {
	h := NewRegistry(&stubRegistry{}, nil)

	rec := httptest.NewRecorder()
	h.Identifiers(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/identifiers", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("код %d, ожидали 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "2AE-A") {
		t.Errorf("в ответе нет записи реестра: %s", rec.Body.String())
	}
}

type stubReview struct {
	acks     map[string]store.Ack
	topology []store.TopologyEntry
}

func (s *stubReview) Acks(ctx context.Context) (map[string]store.Ack, error) { return s.acks, nil }
func (s *stubReview) Unacknowledge(ctx context.Context, edoID, fingerprint string) error {
	delete(s.acks, edoID+"/"+fingerprint)
	return nil
}
func (s *stubReview) All(ctx context.Context) ([]store.TopologyEntry, error) { return s.topology, nil }
func (s *stubReview) Put(ctx context.Context, e store.TopologyEntry) error {
	s.topology = append(s.topology, e)
	return nil
}
func (s *stubReview) Remove(ctx context.Context, inn, kpp, edoID string) error { return nil }

// Скрытые находки — помеченные и погашенные топологией — видны только с all=1.
func TestAnomaliesHidesAckedAndTopologySuppressed(t *testing.T) {
	reg := &stubRegistry{events: []store.AnomalyEvent{
		{ID: 1, EDOID: "A", Kind: "orphan_idle", INN: "7700000001", StateFingerprint: "fa"},
		{ID: 2, EDOID: "B", Kind: "owner_lost", INN: "7700000001", StateFingerprint: "fb"},
		{ID: 3, EDOID: "C", Kind: "owner_lost", INN: "7700000001", StateFingerprint: "fc"},
	}}
	review := &stubReview{
		acks: map[string]store.Ack{"C/fc": {Reason: "ушёл к другому партнёру", ReviewAt: time.Now().Add(-time.Hour)}},
		// Топология гасит только orphan_idle: owner_lost по B остаётся.
		topology: []store.TopologyEntry{{INN: "7700000001", EDOID: "A", Purpose: "запасной"},
			{INN: "7700000001", EDOID: "B", Purpose: "запасной"}},
	}
	h := NewRegistry(reg, nil).WithReview(review, review)

	type item struct {
		EDOID      string `json:"edoId"`
		Acked      bool   `json:"acknowledged"`
		Suppressed bool   `json:"suppressed"`
		ReviewDue  bool   `json:"reviewDue"`
	}
	get := func(url string) []item {
		rec := httptest.NewRecorder()
		h.Anomalies(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, url, nil))
		var body struct {
			Anomalies []item `json:"anomalies"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("ответ %s: %v", rec.Body.String(), err)
		}
		return body.Anomalies
	}
	if active := get("/api/anomalies"); len(active) != 1 || active[0].EDOID != "B" {
		t.Errorf("активные %+v, ожидали только B", active)
	}
	all := get("/api/anomalies?all=1")
	if len(all) != 3 || !all[0].Suppressed || !all[2].Acked || !all[2].ReviewDue {
		t.Errorf("все %+v", all)
	}
}

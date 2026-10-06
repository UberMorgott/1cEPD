package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func testIdentifiers(t *testing.T) *Identifiers {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewIdentifiers(db)
}

func TestUpsertInsertsAndUpdates(t *testing.T) {
	s := testIdentifiers(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)

	rec := IdentifierRecord{
		EDOID: "2AE-A", INN: "7700000001", KPP: "770001001",
		ClientName: "ООО Тест", Login: "user@example.ru",
		OwnerRaw: "CL-1 - Иванов", OwnerCode: "CL-1", Packets: 5,
	}
	if err := s.Upsert(ctx, []IdentifierRecord{rec}, "2026-08", now); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	all, err := s.All(ctx)
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("записей %d, ожидали 1", len(all))
	}
	if !all[0].FirstSeenAt.Equal(now) || !all[0].LastSeenAt.Equal(now) {
		t.Errorf("даты первого и последнего появления должны совпадать при вставке")
	}

	later := now.Add(24 * time.Hour)
	rec.Packets = 9
	if err := s.Upsert(ctx, []IdentifierRecord{rec}, "2026-08", later); err != nil {
		t.Fatalf("повторный Upsert: %v", err)
	}

	all, _ = s.All(ctx)
	if len(all) != 1 {
		t.Fatalf("записей %d, ожидали 1 после обновления", len(all))
	}
	if !all[0].FirstSeenAt.Equal(now) {
		t.Errorf("FirstSeenAt = %v, должен остаться прежним", all[0].FirstSeenAt)
	}
	if !all[0].LastSeenAt.Equal(later) {
		t.Errorf("LastSeenAt = %v, ожидали %v", all[0].LastSeenAt, later)
	}
	if all[0].Packets != 9 {
		t.Errorf("Packets = %d, ожидали 9", all[0].Packets)
	}
}

func TestRecordEventIsIdempotent(t *testing.T) {
	s := testIdentifiers(t)
	ctx := context.Background()
	now := time.Now().UTC()

	event := AnomalyEvent{
		EDOID: "2AE-A", Kind: "orphan_with_traffic", Confidence: "high",
		StateFingerprint: "abc123", INN: "7700000001", Details: "56 пакетов",
	}
	for i := range 3 {
		if err := s.RecordEvent(ctx, event, now); err != nil {
			t.Fatalf("RecordEvent %d: %v", i, err)
		}
	}

	events, err := s.Events(ctx, false)
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("событий %d, ожидали 1: повтор с тем же отпечатком не должен дублироваться", len(events))
	}
}

func TestAckHidesEvent(t *testing.T) {
	s := testIdentifiers(t)
	ctx := context.Background()
	now := time.Now().UTC()

	event := AnomalyEvent{
		EDOID: "2AE-A", Kind: "duplicate", Confidence: "low", StateFingerprint: "fp-1",
	}
	if err := s.RecordEvent(ctx, event, now); err != nil {
		t.Fatalf("RecordEvent: %v", err)
	}

	if err := s.Acknowledge(ctx, "2AE-A", "fp-1", "разные виды деятельности", "admin", now); err != nil {
		t.Fatalf("Acknowledge: %v", err)
	}

	active, err := s.Events(ctx, false)
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	if len(active) != 0 {
		t.Errorf("активных событий %d, подтверждённое показываться не должно", len(active))
	}

	all, err := s.Events(ctx, true)
	if err != nil {
		t.Fatalf("Events(all): %v", err)
	}
	if len(all) != 1 {
		t.Errorf("всего событий %d, ожидали 1", len(all))
	}
	if !all[0].Acknowledged {
		t.Error("событие должно быть помечено подтверждённым")
	}
}

func TestAckDoesNotHideChangedState(t *testing.T) {
	// Подтверждение гасит конкретное состояние. Если состояние изменилось,
	// сигнал обязан появиться снова, иначе одна пометка скроет будущую поломку.
	s := testIdentifiers(t)
	ctx := context.Background()
	now := time.Now().UTC()

	if err := s.RecordEvent(ctx, AnomalyEvent{
		EDOID: "2AE-A", Kind: "duplicate", Confidence: "low", StateFingerprint: "fp-1",
	}, now); err != nil {
		t.Fatalf("RecordEvent: %v", err)
	}
	if err := s.Acknowledge(ctx, "2AE-A", "fp-1", "законно", "admin", now); err != nil {
		t.Fatalf("Acknowledge: %v", err)
	}
	if err := s.RecordEvent(ctx, AnomalyEvent{
		EDOID: "2AE-A", Kind: "duplicate", Confidence: "low", StateFingerprint: "fp-2",
	}, now); err != nil {
		t.Fatalf("RecordEvent с новым отпечатком: %v", err)
	}

	active, _ := s.Events(ctx, false)
	if len(active) != 1 {
		t.Errorf("активных событий %d, ожидали 1: изменившееся состояние должно всплыть", len(active))
	}
}

package store

import (
	"path/filepath"
	"testing"
	"time"

	"partnerops/internal/partner"
)

func TestITSChecksReplaceAndRead(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	s := NewITSChecks(db)
	ctx := t.Context()

	if last, err := s.LastChecked(ctx); err != nil || !last.IsZero() {
		t.Fatalf("пустая база: %v %v", last, err)
	}

	at := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	number := 161
	end := time.Date(2026, 12, 31, 20, 59, 59, 0, time.UTC)
	if err := s.Replace(ctx, []partner.ITSCheck{{SubscriberCode: "CL-OLD", Code: 1}}, at.Add(-time.Hour)); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	checks := []partner.ITSCheck{{
		SubscriberCode: "CL-1000001", Code: 1, Status: "Success",
		Contracts: []partner.ITSContract{{TypeName: "1С:ИТС уровня Проф", TypeNumber: &number, End: end}},
	}}
	if err := s.Replace(ctx, checks, at); err != nil {
		t.Fatalf("Replace: %v", err)
	}

	list, err := s.All(ctx)
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	// Абонент прошлого прогона пропал: в напоминаниях его быть не должно.
	if len(list) != 1 || list[0].SubscriberCode != "CL-1000001" || !list[0].CheckedAt.Equal(at) {
		t.Fatalf("прочитано %+v", list)
	}
	c := list[0].Contracts
	if len(c) != 1 || c[0].TypeNumber == nil || *c[0].TypeNumber != 161 || !c[0].End.Equal(end) {
		t.Errorf("договоры %+v", c)
	}
	if last, _ := s.LastChecked(ctx); !last.Equal(at) {
		t.Errorf("последняя проверка %v, ожидали %v", last, at)
	}
}

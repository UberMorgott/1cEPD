package store

import (
	"path/filepath"
	"testing"
	"time"

	"partnerops/internal/partner"
)

func TestIndustryChecksRoundTrip(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	s := NewIndustryChecks(db)
	ctx := t.Context()

	at := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	end := time.Date(2027, 1, 31, 20, 59, 59, 0, time.UTC)
	checks := []partner.IndustryCheck{{
		SubscriberCode: "CL-1000001", Code: partner.IndustryStatusNeeded,
		Programs: []partner.IndustryProgram{
			{UIN: "p-1", Name: "ЖКХ"},
			{UIN: "p-2", Name: "Медицина", Subscriptions: []partner.IndustrySubscription{{NomenclatureName: "ИТС Отраслевой", End: end}}},
		},
	}}
	if err := s.Replace(ctx, checks, at); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	list, err := s.All(ctx)
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(list) != 1 || !list[0].CheckedAt.Equal(at) || len(list[0].Programs) != 2 {
		t.Fatalf("прочитано %+v", list)
	}
	if missing := list[0].MissingIndustry(); len(missing) != 1 || missing[0].Name != "ЖКХ" {
		t.Errorf("не оформлен %+v", missing)
	}
	if subs := list[0].Programs[1].Subscriptions; len(subs) != 1 || !subs[0].End.Equal(end) {
		t.Errorf("подписки %+v", subs)
	}
	if last, err := s.LastChecked(ctx); err != nil || !last.Equal(at) {
		t.Errorf("последняя проверка %v %v", last, err)
	}
}

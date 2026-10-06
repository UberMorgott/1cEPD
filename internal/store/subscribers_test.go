package store

import (
	"path/filepath"
	"testing"
	"time"

	"partnerops/internal/partner"
)

func TestSubscribersKeepGoneAndUpdateKnown(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	s := NewSubscribers(db)
	ctx := t.Context()

	if last, err := s.LastFetched(ctx); err != nil || !last.IsZero() {
		t.Fatalf("пустая база: %v, %v", last, err)
	}
	first := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	second := first.Add(24 * time.Hour)
	if err := s.Save(ctx, []partner.Subscriber{
		{Code: "CL-1", Name: "Альфа", Subjects: []string{"EDO"},
			Organizations: []partner.Organization{{Name: "ООО Альфа", INN: "7700000001", KPP: "770001001"}}},
		{Code: "CL-2", Name: "Бета"},
	}, first); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := s.Save(ctx, []partner.Subscriber{{Code: "CL-1", Name: "Альфа-2", RegNumbers: []string{"800000001"}}}, second); err != nil {
		t.Fatalf("Save второй: %v", err)
	}

	all, err := s.All(ctx)
	if err != nil || len(all) != 2 {
		t.Fatalf("All: %d, %v", len(all), err)
	}
	alpha, beta := all[0], all[1]
	if alpha.Name != "Альфа-2" || !alpha.FirstSeenAt.Equal(first) || !alpha.LastSeenAt.Equal(second) ||
		len(alpha.RegNumbers) != 1 || len(alpha.Subjects) != 0 || len(alpha.Organizations) != 0 {
		t.Errorf("CL-1 не обновился: %+v", alpha)
	}
	if !beta.LastSeenAt.Equal(first) {
		t.Errorf("пропавший CL-2 должен остаться с прежней датой: %+v", beta)
	}
	if last, _ := s.LastFetched(ctx); !last.Equal(second) {
		t.Errorf("LastFetched = %v, ожидали %v", last, second)
	}
}

package store

import (
	"path/filepath"
	"testing"
	"time"

	"partnerops/internal/partner"
)

func TestOptionReportsSaveAndRead(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	s := NewOptionReports(db)
	ctx := t.Context()

	if fetched, err := s.Fetched(ctx); err != nil || len(fetched) != 0 {
		t.Fatalf("пустая база: %v %v", fetched, err)
	}

	at := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	maxVolume, used := 5.0, 4.0
	end := time.Date(2027, 8, 25, 20, 59, 59, 0, time.UTC)
	report := OptionReport{Type: "REPORTING", State: OptionStateOK, FetchedAt: at, Entries: []partner.OptionEntry{{
		SubscriberCode: "CL-1", Tariffs: []partner.OptionTariff{{Name: "ИТСааС ПРОФ", End: end,
			Options: []partner.Option{{Name: "Лицензия", Quantitative: true, MaxVolume: &maxVolume, UsedVolume: &used},
				{Name: "Качественная"}}}},
	}}}
	if err := s.Save(ctx, report); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := s.Save(ctx, OptionReport{Type: "SIGN", State: OptionStateNone, FetchedAt: at}); err != nil {
		t.Fatalf("Save none: %v", err)
	}
	// Повторное сохранение вида заменяет прошлый отчёт.
	report.FetchedAt = at.Add(time.Hour)
	if err := s.Save(ctx, report); err != nil {
		t.Fatalf("Save again: %v", err)
	}

	list, err := s.All(ctx)
	if err != nil || len(list) != 2 {
		t.Fatalf("All: %d %v", len(list), err)
	}
	got := list[0]
	if got.Type != "REPORTING" || !got.FetchedAt.Equal(at.Add(time.Hour)) || len(got.Entries) != 1 {
		t.Fatalf("отчёт %+v", got)
	}
	tariff := got.Entries[0].Tariffs[0]
	if tariff.TypeNumber != nil || !tariff.End.Equal(end) || len(tariff.Options) != 2 ||
		*tariff.Options[0].UsedVolume != 4 || tariff.Options[1].MaxVolume != nil {
		t.Errorf("тариф %+v", tariff)
	}
	if list[1].State != OptionStateNone || list[1].Entries != nil {
		t.Errorf("пустой вид %+v", list[1])
	}
	fetched, err := s.Fetched(ctx)
	if err != nil || len(fetched) != 2 || !fetched["SIGN"].Equal(at) {
		t.Errorf("Fetched %v %v", fetched, err)
	}
}

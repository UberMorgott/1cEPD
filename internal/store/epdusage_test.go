package store

import (
	"path/filepath"
	"testing"
	"time"

	"partnerops/internal/partner"
)

func TestEPDUsageReplaceKeepsLatestRun(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	s := NewEPDUsage(db)
	ctx := t.Context()

	if _, ok, err := s.Run(ctx); err != nil || ok {
		t.Fatalf("пустая база: ok=%v err=%v", ok, err)
	}
	at := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	old := []partner.EDOTrafficRow{{EDOID: "2AE-OLD", EPDOut: 1}}
	if err := s.Replace(ctx, EPDUsageRun{PeriodFrom: "2025-08", PeriodTo: "2026-07", FetchedAt: at}, old); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	fresh := []partner.EDOTrafficRow{
		{EDOID: "2AE-1", INN: "7700000001", KPP: "770001001", Subscriber: "CL-1", ClientName: "ООО Альфа", EPDIn: 5, EPDOut: 478},
		{EDOID: "2AE-2", INN: "7700000002"},
	}
	run := EPDUsageRun{PeriodFrom: "2025-09", PeriodTo: "2026-08", FetchedAt: at.Add(time.Hour)}
	if err := s.Replace(ctx, run, fresh); err != nil {
		t.Fatalf("Replace: %v", err)
	}

	got, ok, err := s.Run(ctx)
	if err != nil || !ok || got != run {
		t.Fatalf("прогон %+v ok=%v err=%v", got, ok, err)
	}
	rows, err := s.Rows(ctx)
	if err != nil {
		t.Fatalf("Rows: %v", err)
	}
	if len(rows) != 2 || rows[0] != fresh[0] || rows[1].EDOID != "2AE-2" {
		t.Errorf("строки %+v", rows)
	}

	// Отчёт текущего месяца живёт рядом и годовой не затирает.
	month := NewMonthTraffic(db)
	monthRows := []partner.EDOTrafficRow{{
		EDOID: "2AE-1", InvoicesOut: 7, NonInvoicesOut: 3, InvoicesIn: 2, NonInvoicesIn: 1,
		Operator: "Такском", SupportFrom: "2026-09-01", SupportTo: "2027-08-31",
		LinkCreated: "2017-02-15", IDRegistered: "2020-05-21",
	}}
	if err := month.Replace(ctx, EPDUsageRun{PeriodFrom: "2026-09", PeriodTo: "2026-09", FetchedAt: at}, monthRows); err != nil {
		t.Fatalf("Replace месяца: %v", err)
	}
	if got, _ := month.Rows(ctx); len(got) != 1 || got[0] != monthRows[0] {
		t.Errorf("строки месяца %+v", got)
	}
	if got, _ := s.Rows(ctx); len(got) != 2 {
		t.Errorf("годовой отчёт затёрт: %+v", got)
	}
}

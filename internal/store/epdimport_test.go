package store

import (
	"path/filepath"
	"testing"
	"time"

	"partnerops/internal/partner"
)

func TestEPDBillingImportReplaces(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	s := NewEPDBillingImport(db)
	ctx := t.Context()

	if _, ok, err := s.Last(ctx); err != nil || ok {
		t.Fatalf("пустая база: %v, %v", ok, err)
	}
	first := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	if err := s.Replace(ctx, "a.csv", []partner.EPDBillingRow{
		{EDOID: "2AE-1", INN: "7700000001"}, {EDOID: "2AE-2", INN: "7700000002"},
	}, first); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	second := first.Add(time.Hour)
	want := partner.EPDBillingRow{Owner: "FR-FR-1 - Тестов Тест", OwnerCode: "FR-FR-1", OwnerContact: "Тестов Тест",
		Login: "login", EDOID: "2AE-3", ClientName: "ООО Тест", INN: "7700000003", KPP: "770001001",
		ITSTariffs: "тариф", EPDDocs: 7}
	if err := s.Replace(ctx, "b.csv", []partner.EPDBillingRow{want}, second); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	rows, err := s.Rows(ctx)
	if err != nil {
		t.Fatalf("Rows: %v", err)
	}
	if len(rows) != 1 || rows[0] != want {
		t.Errorf("новая загрузка не заменила прежнюю: %+v", rows)
	}
	info, ok, err := s.Last(ctx)
	if err != nil || !ok || !info.ImportedAt.Equal(second) || info.FileName != "b.csv" || info.Total != 1 {
		t.Errorf("сведения о загрузке: %+v %v %v", info, ok, err)
	}
}

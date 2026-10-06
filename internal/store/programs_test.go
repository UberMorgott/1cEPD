package store

import (
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestClientProgramsReplaceKeepsLatestCheck(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	s := NewClientPrograms(db)
	ctx := t.Context()
	at := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)

	old := []ClientProgram{{RegNumber: "1", Program: "Старая", HasAccess: true}}
	if err := s.Replace(ctx, "7811000310", "780001001", old, at.Add(-time.Hour)); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	fresh := []ClientProgram{
		{Login: "buh@romashka.ru", RegNumber: "200001912345", Program: "Бухгалтерия предприятия", HasAccess: true},
		{Login: "buh@romashka.ru", RegNumber: "200001912345", Program: "Технологическая платформа",
			Missing: []string{"Договор 1С:ИТС"}},
	}
	if err := s.Replace(ctx, "7811000310", "780001001", fresh, at); err != nil {
		t.Fatalf("Replace: %v", err)
	}

	list, err := s.All(ctx)
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("строк %d, ожидали 2: %+v", len(list), list)
	}
	if list[0].Program != "Бухгалтерия предприятия" || !list[0].HasAccess || len(list[0].Missing) != 0 ||
		list[0].INN != "7811000310" || list[0].KPP != "780001001" || !list[0].CheckedAt.Equal(at) {
		t.Errorf("первая строка: %+v", list[0])
	}
	if list[1].HasAccess || !slices.Equal(list[1].Missing, []string{"Договор 1С:ИТС"}) {
		t.Errorf("вторая строка: %+v", list[1])
	}
}

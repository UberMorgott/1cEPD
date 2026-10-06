package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"partnerops/internal/partner"
)

func testDB(t *testing.T) *Snapshots {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewSnapshots(db)
}

func sampleRows() []partner.EDOBillingRow {
	limit := int64(100)
	return []partner.EDOBillingRow{
		{EDOID: "2AE-A", INN: "7700000001", Owner: "CL-1", Login: "a", Limit: &limit, Packets: 5},
		{EDOID: "2AE-B", INN: "7700000002", Owner: "", Login: "b"},
	}
}

func TestSaveAndLoadLatest(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	taken := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)

	res, err := s.Save(ctx, "2026-08", taken, []byte("csv"), sampleRows())
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if res.ID == 0 {
		t.Fatal("Save вернул нулевой идентификатор")
	}
	if !res.IsBaseline {
		t.Error("первый снимок периода должен быть baseline")
	}
	if res.Duplicate {
		t.Error("первый снимок не может быть дубликатом")
	}

	snap, rows, err := s.Latest(ctx, "2026-08")
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if snap.ID != res.ID {
		t.Errorf("ID = %d, ожидали %d", snap.ID, res.ID)
	}
	if snap.RowCount != 2 {
		t.Errorf("RowCount = %d, ожидали 2", snap.RowCount)
	}
	if len(rows) != 2 {
		t.Fatalf("строк %d, ожидали 2", len(rows))
	}
	if rows[0].Limit == nil || *rows[0].Limit != 100 {
		t.Errorf("Limit не сохранился: %v", rows[0].Limit)
	}
	if rows[1].Limit != nil {
		t.Errorf("Limit должен быть nil, получили %v", *rows[1].Limit)
	}
}

func TestLatestReturnsNewest(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()

	older := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)

	if _, err := s.Save(ctx, "2026-08", older, []byte("old"), sampleRows()); err != nil {
		t.Fatalf("Save старого: %v", err)
	}
	newRes, err := s.Save(ctx, "2026-08", newer, []byte("new"), sampleRows()[:1])
	if err != nil {
		t.Fatalf("Save нового: %v", err)
	}
	if newRes.IsBaseline {
		t.Error("второй снимок периода не должен быть baseline")
	}

	snap, rows, err := s.Latest(ctx, "2026-08")
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if snap.ID != newRes.ID {
		t.Errorf("вернулся снапшот %d, ожидали %d", snap.ID, newRes.ID)
	}
	if len(rows) != 1 {
		t.Errorf("строк %d, ожидали 1", len(rows))
	}
}

func TestSaveSkipsIdenticalReport(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	raw := []byte("одинаковый csv")

	first, err := s.Save(ctx, "2026-08", time.Now(), raw, sampleRows())
	if err != nil {
		t.Fatalf("первый Save: %v", err)
	}

	second, err := s.Save(ctx, "2026-08", time.Now().Add(time.Hour), raw, sampleRows())
	if err != nil {
		t.Fatalf("второй Save: %v", err)
	}
	if !second.Duplicate {
		t.Error("повторный идентичный отчёт должен помечаться как дубликат")
	}
	if second.ID != first.ID {
		t.Errorf("дубликат вернул id %d, ожидали %d", second.ID, first.ID)
	}

	var count int
	if err := s.db.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM edo_snapshots`).Scan(&count); err != nil {
		t.Fatalf("подсчёт снапшотов: %v", err)
	}
	if count != 1 {
		t.Errorf("снапшотов в базе %d, ожидали 1", count)
	}
}

// Совпавший отчёт снимка не создаёт, но время построения сдвигает:
// по нему старт сервиса решает, строить ли отчёт снова.
func TestLastFetchedMovesOnDuplicate(t *testing.T) {
	s := testDB(t)
	ctx := t.Context()

	if last, err := s.LastFetched(ctx, "2026-08"); err != nil || !last.IsZero() {
		t.Fatalf("пустая база: %v, %v", last, err)
	}
	first := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	second := first.Add(26 * time.Hour)
	if _, err := s.Save(ctx, "2026-08", first, []byte("csv"), sampleRows()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := s.Save(ctx, "2026-08", second, []byte("csv"), sampleRows()); err != nil {
		t.Fatalf("Save дубликата: %v", err)
	}
	last, err := s.LastFetched(ctx, "2026-08")
	if err != nil {
		t.Fatalf("LastFetched: %v", err)
	}
	if !last.Equal(second) {
		t.Errorf("LastFetched = %v, ожидали %v", last, second)
	}
}

func TestSaveDistinguishesPeriods(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	raw := []byte("одинаковый csv")

	if _, err := s.Save(ctx, "2026-07", time.Now(), raw, sampleRows()); err != nil {
		t.Fatalf("Save за июль: %v", err)
	}
	res, err := s.Save(ctx, "2026-08", time.Now(), raw, sampleRows())
	if err != nil {
		t.Fatalf("Save за август: %v", err)
	}
	if res.Duplicate {
		t.Error("одинаковый CSV в разных периодах — не дубликат")
	}
	if !res.IsBaseline {
		t.Error("первый снимок августа должен быть baseline")
	}
}

func TestLatestOnEmptyDatabase(t *testing.T) {
	s := testDB(t)
	_, _, err := s.Latest(context.Background(), "2026-08")
	if !errors.Is(err, ErrNoSnapshot) {
		t.Errorf("err = %v, ожидали ErrNoSnapshot", err)
	}
}

func TestSaveIsAtomic(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()

	// Строка с недопустимо длинным значением не должна оставить пустой снапшот.
	rows := sampleRows()
	if _, err := s.Save(ctx, "2026-08", time.Now(), []byte("csv"), rows); err != nil {
		t.Fatalf("Save: %v", err)
	}

	snap, _, err := s.Latest(ctx, "2026-08")
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if snap.RowCount != int64(len(rows)) {
		t.Errorf("RowCount = %d, ожидали %d", snap.RowCount, len(rows))
	}
}

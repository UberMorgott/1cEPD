package store

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testRequests(t *testing.T) *Requests {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewRequests(db)
}

func TestSaveAndLoadDraft(t *testing.T) {
	s := testRequests(t)
	ctx := context.Background()

	id, err := s.Save(ctx, RequestDraft{
		Title:       "ЭПД-10000 для Тест",
		Status:      "draft",
		SchemaVer:   "3.09",
		PayloadJSON: `{"partnerCode":"00000"}`,
	}, time.Now())
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	draft, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if draft.Title != "ЭПД-10000 для Тест" {
		t.Errorf("название %q", draft.Title)
	}
	if draft.Revision != 1 {
		t.Errorf("ревизия %d, ожидали 1", draft.Revision)
	}
}

func TestSaveNumbersRequestsInOrder(t *testing.T) {
	s := testRequests(t)
	ctx := context.Background()
	for want := int64(1); want <= 3; want++ {
		id, err := s.Save(ctx, RequestDraft{Status: "draft", SchemaVer: "3.09", PayloadJSON: `{}`}, time.Now())
		if err != nil {
			t.Fatalf("Save: %v", err)
		}
		draft, err := s.Get(ctx, id)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if draft.Number != want {
			t.Errorf("номер %d, ожидали %d", draft.Number, want)
		}
	}
}

// Миграция 019 нумерует старые заявки по порядку создания и вписывает номер в
// название по старому образцу; своё название остаётся.
func TestRequestNumbersBackfill(t *testing.T) {
	s := testRequests(t)
	ctx := context.Background()
	insert := `INSERT INTO its_requests (number, title, payload_json, created_at, updated_at) VALUES (0, ?, '{}', ?, ?)`
	for _, row := range []struct {
		title   string
		created int64
	}{
		{"Заявка 00000 от 03.09.2026", 300},
		{"Своё название", 100},
		{"Заявка — от 02.09.2026", 200},
	} {
		if _, err := s.db.ExecContext(ctx, insert, row.title, row.created, row.created); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	body, err := migrationFiles.ReadFile("migrations/019_request_numbers.sql")
	if err != nil {
		t.Fatalf("миграция: %v", err)
	}
	backfill := strings.SplitN(string(body), "DEFAULT 0;", 2)[1]
	if _, err := s.db.ExecContext(ctx, backfill); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	list, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	got := map[int64]string{}
	for _, draft := range list {
		got[draft.Number] = draft.Title
	}
	want := map[int64]string{1: "Своё название", 2: "Заявка 2 от 02.09.2026", 3: "Заявка 3 от 03.09.2026"}
	for number, title := range want {
		if got[number] != title {
			t.Errorf("заявка №%d: %q, ожидали %q", number, got[number], title)
		}
	}

	id, err := s.Save(ctx, RequestDraft{Status: "draft", SchemaVer: "3.09", PayloadJSON: `{}`}, time.Now())
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if draft, _ := s.Get(ctx, id); draft.Number != 4 {
		t.Errorf("новая заявка №%d, ожидали 4", draft.Number)
	}
}

func TestUpdateBumpsRevision(t *testing.T) {
	s := testRequests(t)
	ctx := context.Background()

	id, err := s.Save(ctx, RequestDraft{Title: "первая", Status: "draft", SchemaVer: "3.09",
		PayloadJSON: "{}"}, time.Now())
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	draft, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	draft.Title = "вторая"
	if err := s.Update(ctx, draft, time.Now()); err != nil {
		t.Fatalf("Update: %v", err)
	}

	updated, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get после обновления: %v", err)
	}
	if updated.Revision != 2 {
		t.Errorf("ревизия %d, ожидали 2", updated.Revision)
	}
	if updated.Title != "вторая" {
		t.Errorf("название не обновилось: %q", updated.Title)
	}
}

func TestUpdateRejectsStaleRevision(t *testing.T) {
	// Двое редактируют один черновик: второй не должен затирать чужие правки молча.
	s := testRequests(t)
	ctx := context.Background()

	id, err := s.Save(ctx, RequestDraft{Title: "исходная", Status: "draft", SchemaVer: "3.09",
		PayloadJSON: "{}"}, time.Now())
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	first, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	second, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	first.Title = "правка первого"
	if err := s.Update(ctx, first, time.Now()); err != nil {
		t.Fatalf("первое обновление: %v", err)
	}

	second.Title = "правка второго"
	if err := s.Update(ctx, second, time.Now()); !errors.Is(err, ErrStaleRevision) {
		t.Errorf("err = %v, ожидали ErrStaleRevision", err)
	}
}

func TestListReturnsNewestFirst(t *testing.T) {
	s := testRequests(t)
	ctx := context.Background()
	now := time.Now()

	if _, err := s.Save(ctx, RequestDraft{Title: "старая", Status: "draft", SchemaVer: "3.09",
		PayloadJSON: "{}"}, now); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := s.Save(ctx, RequestDraft{Title: "новая", Status: "draft", SchemaVer: "3.09",
		PayloadJSON: "{}"}, now.Add(time.Hour)); err != nil {
		t.Fatalf("Save: %v", err)
	}

	list, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 || list[0].Title != "новая" {
		t.Errorf("порядок нарушен: %+v", list)
	}
}

func TestMarkExported(t *testing.T) {
	s := testRequests(t)
	ctx := context.Background()
	id, err := s.Save(ctx, RequestDraft{Title: "заявка", Status: "draft", SchemaVer: "3.09",
		PayloadJSON: "{}"}, time.Now())
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := s.MarkExported(ctx, id, time.Now()); err != nil {
		t.Fatalf("MarkExported: %v", err)
	}

	draft, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if draft.Status != "exported" {
		t.Errorf("статус %q, ожидали exported", draft.Status)
	}
	if draft.ExportedAt == nil {
		t.Error("дата выгрузки не проставлена")
	}
}

func TestPayloadsNewestFirst(t *testing.T) {
	s := testRequests(t)
	ctx := context.Background()
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

	for i, payload := range []string{`{"n":1}`, `{"n":2}`} {
		if _, err := s.Save(ctx, RequestDraft{Status: "draft", SchemaVer: "3.09", PayloadJSON: payload},
			start.Add(time.Duration(i)*time.Hour)); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}

	got, err := s.Payloads(ctx)
	if err != nil {
		t.Fatalf("Payloads: %v", err)
	}
	if len(got) != 2 || got[0] != `{"n":2}` || got[1] != `{"n":1}` {
		t.Errorf("Payloads = %v, ожидали свежую первой", got)
	}
}

package service

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"partnerops/internal/partner"
	"partnerops/internal/store"
)

type scriptedReports struct {
	dates   []time.Time
	answers []error // nil — отдать sampleCSV
}

func (f *scriptedReports) EDOBillingReport(ctx context.Context, date time.Time) ([]byte, error) {
	f.dates = append(f.dates, date)
	var err error
	if len(f.answers) > 0 {
		err, f.answers = f.answers[0], f.answers[1:]
	}
	if err != nil {
		return nil, err
	}
	return []byte(sampleCSV), nil
}

func TestBillingBackfillOneMonthPerIntervalNewestFirst(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx := t.Context()
	snapshots, attempts := store.NewSnapshots(db), store.NewBackfill(db)

	reports := &scriptedReports{answers: []error{
		nil,
		&partner.APIError{StatusCode: 400, Code: "BILLING_DOES_NOT_EXIST"},
		&partner.APIError{StatusCode: 400, Code: "MAX_TASKS_PER_HOUR_LIMIT_REACHED"},
		nil,
	}}
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	backfill := NewBillingBackfill(reports, snapshots, attempts, nil)
	backfill.now = func() time.Time { return now }

	step := func(want string) {
		t.Helper()
		got, _ := backfill.Step(ctx)
		if got != want {
			t.Fatalf("шаг в %s: месяц %q, ждали %q", now.Format(time.TimeOnly), got, want)
		}
	}

	step("2026-08")
	step("") // интервал не прошёл — отчёт не строим
	now = now.Add(BackfillInterval)
	step("2026-07") // биллинга нет — месяц больше не просим
	now = now.Add(BackfillInterval)
	step("2026-06") // лимит 1С — повтор через интервал
	now = now.Add(BackfillInterval)
	step("2026-06")
	if len(reports.dates) != 4 || !reports.dates[0].Equal(time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("отчёты %v", reports.dates)
	}

	// Новый сервис поверх той же базы: пауза после перезапуска не сбрасывается.
	restarted := NewBillingBackfill(reports, snapshots, attempts, nil)
	restarted.now = func() time.Time { return now.Add(time.Minute) }
	if got, _ := restarted.Step(ctx); got != "" {
		t.Errorf("после перезапуска отчёт раньше интервала: %q", got)
	}

	saved, err := attempts.Attempts(ctx)
	if err != nil {
		t.Fatalf("Attempts: %v", err)
	}
	if saved["2026-07"].Status != store.BackfillNone || saved["2026-06"].Status != store.BackfillDone ||
		saved["2026-06"].Failures != 0 {
		t.Errorf("попытки %+v", saved)
	}
}

func TestBillingBackfillGivesUpAfterRepeatedFailures(t *testing.T) {
	months := ClosedMonths(time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC))
	if len(months) != 12 || months[0].Format("2006-01") != "2025-09" || months[11].Format("2006-01") != "2026-08" {
		t.Fatalf("месяцы %v", months)
	}
	attempts := map[string]store.BackfillAttempt{
		"2026-08": {Status: store.BackfillFailed, Failures: backfillMaxFailures},
		"2026-07": {Status: store.BackfillNone},
	}
	next, ok := nextBackfillMonth(months, map[string]bool{"2026-06": true}, attempts)
	if !ok || next.Format("2006-01") != "2026-05" {
		t.Errorf("следующий месяц %v %v", next, ok)
	}
}

func TestBuildBillingHistoryAlignsClientsWithMonths(t *testing.T) {
	limit := int64(100)
	rows := map[string][]partner.EDOBillingRow{
		"2026-07": {{EDOID: "2AE-A", Packets: 40, Limit: &limit}},
		"2026-08": {
			{EDOID: "2AE-A", Packets: 120, Limit: &limit, PacketsBillable: 20, ClientAmount: 140_000},
			{EDOID: "2AE-B", Packets: 3},
		},
	}
	attempts := map[string]store.BackfillAttempt{"2026-06": {Status: store.BackfillNone}}
	history := BuildBillingHistory([]string{"2026-05", "2026-06", "2026-07", "2026-08"}, rows, attempts)

	statuses := []string{HistoryPending, HistoryNone, HistoryOK, HistoryOK}
	for i, month := range history.Months {
		if month.Status != statuses[i] {
			t.Errorf("месяц %s: статус %s", month.Period, month.Status)
		}
	}
	if aug := history.Months[3]; aug.Packets != 123 || aug.Billable != 20 || aug.Clients != 2 || aug.TotalDue != "140,00" {
		t.Errorf("итоги августа %+v", aug)
	}
	if len(history.Clients) != 2 {
		t.Fatalf("клиенты %+v", history.Clients)
	}
	a := history.Clients[0]
	if a.EDOID != "2AE-A" || a.Packets[0] != nil || *a.Packets[2] != 40 || *a.Packets[3] != 120 ||
		*a.Limits[3] != 100 || a.Billable[3] != 20 {
		t.Errorf("клиент A %+v", a)
	}
	if b := history.Clients[1]; b.Packets[2] != nil || *b.Packets[3] != 3 {
		t.Errorf("клиент B %+v", b)
	}
}

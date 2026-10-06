package service

import (
	"testing"
	"time"

	"partnerops/internal/partner"
	"partnerops/internal/store"
)

func TestMonthTrafficRefresherOncePerDayFromMonthStart(t *testing.T) {
	source := &fakeTraffic{}
	saved := &fakeUsageStore{}
	refresher := NewMonthTrafficRefresher(source, saved)
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	refresher.now = func() time.Time { return now }

	if called, err := refresher.Refresh(t.Context()); err != nil || !called {
		t.Fatalf("первый прогон: called=%v err=%v", called, err)
	}
	if !source.from.Equal(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)) || !source.to.Equal(now) {
		t.Errorf("период %v…%v", source.from, source.to)
	}
	if saved.run.PeriodFrom != "2026-09" || saved.rows[0].InvoicesOut != 0 || saved.rows[0].EPDOut != 478 {
		t.Errorf("сохранено %+v %+v", saved.run, saved.rows)
	}
	now = now.Add(6 * time.Hour) // перезапуск в тот же день — лимит 1С не тратим
	if called, _ := refresher.Refresh(t.Context()); called || source.calls != 1 {
		t.Errorf("повторный отчёт: calls=%d", source.calls)
	}
	now = now.Add(monthTrafficEvery)
	if called, _ := refresher.Refresh(t.Context()); !called || source.calls != 2 {
		t.Errorf("суточный прогон не обновил отчёт: calls=%d", source.calls)
	}
}

func TestBuildForecastExtrapolatesToMonthEnd(t *testing.T) {
	limit, small := int64(100), int64(50)
	registry := []store.IdentifierRecord{
		{EDOID: "2AE-GROWS", ClientName: "ООО Альфа", Limit: &limit},
		{EDOID: "2AE-OVER", ClientName: "ООО Бета", Limit: &small},
		{EDOID: "2AE-FINE", Limit: &limit},
		{EDOID: "2AE-NOLIMIT"},
	}
	rows := []partner.EDOTrafficRow{
		{EDOID: "2AE-GROWS", InvoicesOut: 40}, {EDOID: "2AE-GROWS", InvoicesOut: 20},
		{EDOID: "2AE-OVER", InvoicesOut: 51},
		{EDOID: "2AE-FINE", InvoicesOut: 10},
		{EDOID: "2AE-NOLIMIT", InvoicesOut: 500},
	}
	// Прошло 15 из 30 дней сентября: расход удваивается.
	run := store.EPDUsageRun{PeriodFrom: "2026-09", FetchedAt: time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)}
	forecast := BuildForecast(run, rows, registry)

	if forecast.DaysInMonth != 30 || forecast.DaysElapsed != 15 || len(forecast.Clients) != 3 {
		t.Fatalf("прогноз %+v", forecast)
	}
	over, grows, fine := forecast.Clients[0], forecast.Clients[1], forecast.Clients[2]
	if over.EDOID != "2AE-OVER" || !over.Exceeded || over.Projected != 102 || over.OverBy != 52 {
		t.Errorf("сверх лимита %+v", over)
	}
	if grows.EDOID != "2AE-GROWS" || grows.Used != 60 || grows.Projected != 120 || grows.OverBy != 20 || !grows.Warn {
		t.Errorf("растущий %+v", grows)
	}
	if fine.Warn || fine.OverBy != 0 {
		t.Errorf("в пределах лимита %+v", fine)
	}

	// Второй день месяца: экстраполировать рано, предупреждаем только о превышенном.
	run.FetchedAt = time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	for _, f := range BuildForecast(run, rows, registry).Clients {
		if f.Warn != f.Exceeded {
			t.Errorf("ранний прогноз %+v", f)
		}
	}
}

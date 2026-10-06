package service

import (
	"context"
	"testing"
	"time"

	"partnerops/internal/partner"
	"partnerops/internal/store"
)

// trafficCSV — заголовок живого отчёта трафика с обезличенными строками.
const trafficCSV = "\xef\xbb\xbfНазвание организации;ИНН;КПП;Абонент;Оператор;Дата сопровождения с;" +
	"Дата сопровождения по;Идентификатор ЭДО;Логин;Дата создания связи;" +
	"Дата регистрации идентификатора ЭДО;СФ (вх.);не_СФ (вх.);ЭПД (вх.);СФ (исх.);не_СФ (исх.);ЭПД (исх.)\n" +
	"ООО Альфа;7700000001;770001001;CL-1000001 - Альфа;Оператор;;;2AE-1;a@example.ru;;;0;0;50;0;0;478\n"

type fakeTraffic struct {
	calls    int
	from, to time.Time
}

func (f *fakeTraffic) EDOTrafficReport(ctx context.Context, from, to time.Time) ([]byte, error) {
	f.calls++
	f.from, f.to = from, to
	return []byte(trafficCSV), nil
}

type fakeUsageStore struct {
	run  store.EPDUsageRun
	ok   bool
	rows []partner.EDOTrafficRow
}

func (f *fakeUsageStore) Replace(ctx context.Context, run store.EPDUsageRun, rows []partner.EDOTrafficRow) error {
	f.run, f.ok, f.rows = run, true, rows
	return nil
}

func (f *fakeUsageStore) Run(ctx context.Context) (store.EPDUsageRun, bool, error) {
	return f.run, f.ok, nil
}

func TestEPDUsageRefresherBuildsOncePerMonth(t *testing.T) {
	source := &fakeTraffic{}
	saved := &fakeUsageStore{}
	refresher := NewEPDUsageRefresher(source, saved)
	refresher.now = func() time.Time { return time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC) }

	if called, err := refresher.Refresh(t.Context()); err != nil || !called {
		t.Fatalf("первый прогон: called=%v err=%v", called, err)
	}
	if !source.from.Equal(time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC)) ||
		!source.to.Equal(time.Date(2026, 8, 31, 23, 59, 59, 0, time.UTC)) {
		t.Errorf("период %v…%v", source.from, source.to)
	}
	if saved.run.PeriodFrom != "2025-09" || saved.run.PeriodTo != "2026-08" || len(saved.rows) != 1 ||
		saved.rows[0].EPDOut != 478 {
		t.Errorf("сохранено %+v %+v", saved.run, saved.rows)
	}

	// Тот же месяц — отчёт не строим: он тратит часовой лимит.
	if called, _ := refresher.Refresh(t.Context()); called || source.calls != 1 {
		t.Errorf("повторный отчёт в том же месяце: calls=%d", source.calls)
	}
	refresher.now = func() time.Time { return time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC) }
	if called, _ := refresher.Refresh(t.Context()); !called || source.calls != 2 {
		t.Errorf("новый месяц не обновил отчёт: calls=%d", source.calls)
	}
}

func TestBuildEPDAdviceComparesWithHeldTariff(t *testing.T) {
	rows := []partner.EDOTrafficRow{
		// Два идентификатора одной организации: расход складывается.
		{INN: "7700000001", KPP: "770001001", ClientName: "ООО Альфа", Subscriber: "CL-1 - Альфа", EPDOut: 600},
		{INN: "7700000001", KPP: "770001001", Subscriber: "CL-1 - Альфа", EPDOut: 400},
		{INN: "7700000002", ClientName: "ИП Бета", Subscriber: "FR-FR-2", EPDOut: 478},
		{INN: "7700000003", ClientName: "ООО Ноль"},
	}
	registry := []store.IdentifierRecord{
		// Строка тарифов повторяется у двух идентификаторов — подписка одна.
		{INN: "7700000001", KPP: "770001001", ITSTariffs: "1С-ЭДО. ЭПД-600(0): подп. ИТС №: 1"},
		{INN: "7700000001", KPP: "770001001", ITSTariffs: "1С-ЭДО. ЭПД-600(0): подп. ИТС №: 1"},
	}

	advice := BuildEPDAdvice(rows, registry)
	if len(advice) != 2 {
		t.Fatalf("подсказок %d, ожидали 2 (без расхода и тарифа — пропуск): %+v", len(advice), advice)
	}
	alpha := advice[0]
	// 1000 документов: ЭПД-600 + 400 поштучно = 3 600 + 2 800 = 6 400 ₽,
	// ЭПД-1000 = 5 000 ₽, выгода 1 400 ₽.
	if alpha.EPDOut != 1000 || alpha.CurrentCostKopeks != 640000 || !alpha.BestOK ||
		alpha.Best.Code != "2081" || alpha.SavingsKopeks != 140000 || alpha.Fresh {
		t.Errorf("Альфа: %+v", alpha)
	}
	beta := advice[1]
	// 478 поштучно = 3 346 ₽; ЭПД-200 даёт ровно столько же — пакет не предлагаем.
	if beta.BestOK || beta.BestCostKopeks != 334600 || !beta.AlreadyOptimal || !beta.Fresh {
		t.Errorf("Бета: %+v", beta)
	}
}

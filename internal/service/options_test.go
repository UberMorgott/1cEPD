package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"partnerops/internal/partner"
	"partnerops/internal/store"
)

type fakeOptionsSource struct {
	calls []string
	err   map[string]error
}

func (f *fakeOptionsSource) OptionBillingReport(ctx context.Context, kind string) (partner.OptionReport, error) {
	f.calls = append(f.calls, kind)
	if err := f.err[kind]; err != nil {
		return partner.OptionReport{}, err
	}
	return partner.OptionReport{State: partner.OptionReportOK,
		Entries: []partner.OptionEntry{{SubscriberCode: "CL-1"}}}, nil
}

type fakeOptionsStore struct{ saved map[string]store.OptionReport }

func (f *fakeOptionsStore) Save(ctx context.Context, report store.OptionReport) error {
	f.saved[report.Type] = report
	return nil
}

func (f *fakeOptionsStore) Fetched(ctx context.Context) (map[string]time.Time, error) {
	result := map[string]time.Time{}
	for kind, report := range f.saved {
		result[kind] = report.FetchedAt
	}
	return result, nil
}

func TestOptionsStepBuildsOneStaleTypeAtATime(t *testing.T) {
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	source := &fakeOptionsSource{err: map[string]error{
		"SIGN": &partner.APIError{StatusCode: 400, Code: "BILLING_DOES_NOT_EXIST"},
	}}
	saved := &fakeOptionsStore{saved: map[string]store.OptionReport{
		// Свежий отчёт не перестраивается, старый — да.
		"REPORTING":    {Type: "REPORTING", FetchedAt: now.Add(-time.Hour)},
		"CLOUD_BACKUP": {Type: "CLOUD_BACKUP", FetchedAt: now.Add(-OptionsRefreshInterval)},
	}}
	r := NewOptionsRefresher(source, saved)
	r.now = func() time.Time { return now }

	for range 2 {
		if called, err := r.Step(t.Context()); !called || err != nil {
			t.Fatalf("шаг: %v %v", called, err)
		}
	}
	if len(source.calls) != 2 || source.calls[0] != "SIGN" || source.calls[1] != "CLOUD_BACKUP" {
		t.Fatalf("заказаны %v", source.calls)
	}
	if saved.saved["SIGN"].State != store.OptionStateNone || saved.saved["CLOUD_BACKUP"].State != store.OptionStateOK ||
		len(saved.saved["CLOUD_BACKUP"].Entries) != 1 {
		t.Errorf("сохранено %+v", saved.saved)
	}
}

func TestOptionsStepPausesOnRateLimit(t *testing.T) {
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	source := &fakeOptionsSource{err: map[string]error{
		"REPORTING": &partner.APIError{StatusCode: 400, Code: "MAX_TASKS_PER_HOUR_LIMIT_REACHED"},
	}}
	saved := &fakeOptionsStore{saved: map[string]store.OptionReport{}}
	r := NewOptionsRefresher(source, saved)
	r.now = func() time.Time { return now }

	if called, err := r.Step(t.Context()); !called || !partner.IsRateLimited(errors.Unwrap(err)) {
		t.Fatalf("лимит: %v %v", called, err)
	}
	// Через 10 минут — пауза, в 1С не ходим.
	now = now.Add(10 * time.Minute)
	if called, _ := r.Step(t.Context()); called || len(source.calls) != 1 {
		t.Fatalf("во время паузы заказан отчёт: %v", source.calls)
	}
	now = now.Add(OptionsRateLimitPause)
	delete(source.err, "REPORTING")
	if called, err := r.Step(t.Context()); !called || err != nil || saved.saved["REPORTING"].State != store.OptionStateOK {
		t.Fatalf("после паузы: %v %v", called, err)
	}
	if _, ok := saved.saved["SIGN"]; ok {
		t.Error("за шаг построено больше одного отчёта")
	}
}

func licenseFixture(now time.Time) []store.OptionReport {
	five, one, ten := 5.0, 1.0, 10.0
	used5, used1, used9 := 5.0, 1.0, 9.0
	soon := now.AddDate(0, 0, 10)
	later := now.AddDate(1, 0, 0)
	itsaas := partner.OptionTariff{Name: "ИТСааС ПРОФ", OrgINN: "7700000001", End: soon}
	renewedOld := partner.OptionTariff{Name: "Облачный архив", OrgINN: "7700000002", End: soon,
		Options: []partner.Option{{Name: "Место", Quantitative: true, MaxVolume: &ten, UsedVolume: &used9}}}
	renewedNew := renewedOld
	renewedNew.End = later
	reporting := itsaas
	reporting.Options = []partner.Option{{Name: "Лицензия", Quantitative: true, MaxVolume: &one, UsedVolume: &used1}}
	sign := itsaas
	sign.Options = []partner.Option{{Name: "Подписи", Quantitative: true, MaxVolume: &five, UsedVolume: &used5},
		{Name: "Качественная"}}
	return []store.OptionReport{
		{Type: "REPORTING", Entries: []partner.OptionEntry{{SubscriberCode: "CL-1", Tariffs: []partner.OptionTariff{reporting}}}},
		{Type: "SIGN", Entries: []partner.OptionEntry{{SubscriberCode: "CL-1", Tariffs: []partner.OptionTariff{sign}}}},
		{Type: "CLOUD_BACKUP", Entries: []partner.OptionEntry{{SubscriberCode: "CL-2",
			Tariffs: []partner.OptionTariff{renewedOld, renewedNew}}}},
	}
}

func TestLicenseTariffsMergeTypes(t *testing.T) {
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	tariffs := LicenseTariffs(licenseFixture(now))
	if len(tariffs["CL-1"]) != 1 || len(tariffs["CL-1"][0].Options) != 3 {
		t.Fatalf("один тариф из двух отчётов: %+v", tariffs["CL-1"])
	}
	if tariffs["CL-1"][0].Options[0].Type != "REPORTING" || tariffs["CL-1"][0].Options[1].Type != "SIGN" {
		t.Errorf("виды опций %+v", tariffs["CL-1"][0].Options)
	}

	expiring := ExpiringLicenses(tariffs, now, 30)
	// CL-2 продлён — напоминания нет.
	if len(expiring) != 1 || expiring[0].Tariff.SubscriberCode != "CL-1" || expiring[0].DaysLeft != 10 {
		t.Fatalf("кончаются: %+v", expiring)
	}
	if got := ExpiringLicenses(tariffs, now, 5); len(got) != 0 {
		t.Errorf("окно 5 дн.: %+v", got)
	}

	low := LowLicenses(tariffs, now)
	// «1 из 1» — не повод; «5 из 5» и «9 из 10» — мало остатка.
	if len(low) != 2 || low[0].Option.Name != "Подписи" || low[0].Remaining != 0 || low[0].Over ||
		low[1].Option.Name != "Место" {
		t.Fatalf("мало остатка: %+v", low)
	}
}

package service

import (
	"context"
	"testing"
	"time"

	"partnerops/internal/partner"
	"partnerops/internal/store"
)

func contract(name string, end time.Time) partner.ITSContract {
	return partner.ITSContract{TypeName: name, End: end}
}

func TestExpiringContractsSkipsRenewedAndFar(t *testing.T) {
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	checks := []store.ITSCheck{
		{SubscriberCode: "CL-1", Contracts: []partner.ITSContract{
			// Кончается завтра, но продолжение уже оформлено — не напоминаем.
			contract("Базовый", now.Add(33*time.Hour)),
			contract("Базовый", now.AddDate(0, 6, 0)),
			// Другой вид договора без продолжения — напоминаем.
			contract("Аренда", now.AddDate(0, 0, 10)),
		}},
		{SubscriberCode: "CL-2", Contracts: []partner.ITSContract{
			contract("Проф", now.AddDate(0, 0, 45)),       // за окном
			contract("Отчётность", now.AddDate(0, 0, -3)), // только что истёк
			contract("Старый", now.AddDate(-1, 0, 0)),     // давно истёк
		}},
	}

	got := ExpiringContracts(checks, now, 30)
	if len(got) != 2 {
		t.Fatalf("найдено %d, ожидали 2: %+v", len(got), got)
	}
	if got[0].Contract.TypeName != "Отчётность" || got[0].DaysLeft != -3 {
		t.Errorf("первый: %+v", got[0])
	}
	if got[1].SubscriberCode != "CL-1" || got[1].Contract.TypeName != "Аренда" || got[1].DaysLeft != 10 {
		t.Errorf("второй: %+v", got[1])
	}
}

type fakeITSSource struct {
	calls int
	codes []string
}

func (f *fakeITSSource) CheckITSBySubscriberCodes(ctx context.Context, codes []string) ([]partner.ITSCheck, error) {
	f.calls++
	f.codes = codes
	return []partner.ITSCheck{{SubscriberCode: codes[0], Code: 1}}, nil
}

type fakeITSStore struct {
	last  time.Time
	saved []partner.ITSCheck
}

func (f *fakeITSStore) Replace(ctx context.Context, checks []partner.ITSCheck, at time.Time) error {
	f.saved, f.last = checks, at
	return nil
}

func (f *fakeITSStore) LastChecked(ctx context.Context) (time.Time, error) { return f.last, nil }

type fakeOwners []store.IdentifierRecord

func (f fakeOwners) All(ctx context.Context) ([]store.IdentifierRecord, error) { return f, nil }

func TestITSRefresherSkipsFreshCheck(t *testing.T) {
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	source := &fakeITSSource{}
	saved := &fakeITSStore{last: now.Add(-time.Hour)}
	owners := fakeOwners{{OwnerCode: "FR-FR-2"}, {OwnerCode: "CL-1"}, {OwnerCode: "CL-1"}, {OwnerCode: ""}}
	refresher := NewITSRefresher(source, saved, owners)
	refresher.now = func() time.Time { return now }

	called, err := refresher.Refresh(t.Context(), ITSRefreshInterval)
	if err != nil || called || source.calls != 0 {
		t.Fatalf("свежая проверка повторена: called=%v calls=%d err=%v", called, source.calls, err)
	}

	called, err = refresher.Refresh(t.Context(), time.Minute)
	if err != nil || !called || source.calls != 1 {
		t.Fatalf("устаревшая проверка не повторена: called=%v calls=%d err=%v", called, source.calls, err)
	}
	if len(source.codes) != 2 || source.codes[0] != "CL-1" || source.codes[1] != "FR-FR-2" {
		t.Errorf("коды %v: нужны без повторов и пустых", source.codes)
	}
	if !saved.last.Equal(now) || len(saved.saved) != 1 {
		t.Errorf("сохранено %+v в %v", saved.saved, saved.last)
	}
}

type fakeIndustry struct {
	calls int
	last  time.Time
}

func (f *fakeIndustry) CheckIndustryBySubscriberCodes(
	ctx context.Context, codes []string,
) ([]partner.IndustryCheck, error) {
	f.calls++
	return []partner.IndustryCheck{{SubscriberCode: codes[0], Code: 106}}, nil
}

func (f *fakeIndustry) Replace(ctx context.Context, checks []partner.IndustryCheck, at time.Time) error {
	f.last = at
	return nil
}

func (f *fakeIndustry) LastChecked(ctx context.Context) (time.Time, error) { return f.last, nil }

func TestITSRefresherChecksIndustryNeverChecked(t *testing.T) {
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	source := &fakeITSSource{}
	// Договоры ИТС свежие, а ИТС Отраслевой не проверялся ни разу — прогон нужен.
	saved := &fakeITSStore{last: now.Add(-time.Hour)}
	industry := &fakeIndustry{}
	refresher := NewITSRefresher(source, saved, fakeOwners{{OwnerCode: "CL-1"}}).WithIndustry(industry, industry)
	refresher.now = func() time.Time { return now }

	called, err := refresher.Refresh(t.Context(), ITSRefreshInterval)
	if err != nil || !called || industry.calls != 1 || !industry.last.Equal(now) {
		t.Fatalf("called=%v err=%v calls=%d last=%v", called, err, industry.calls, industry.last)
	}
	if called, _ := refresher.Refresh(t.Context(), ITSRefreshInterval); called {
		t.Error("обе проверки свежие, а прогон повторён")
	}
}

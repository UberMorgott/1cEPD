package service

import (
	"context"
	"slices"
	"testing"
	"time"

	"partnerops/internal/partner"
	"partnerops/internal/store"
)

func TestCheckSubscribersFindsGapsBothWays(t *testing.T) {
	subs := []store.SubscriberRecord{
		// В биллинге по коду владельца.
		{Code: "CL-1", Subjects: []string{"EDO"}},
		// Направление EDO есть, в биллинге нет — «ЭДО без биллинга».
		{Code: "CL-2", Subjects: []string{"EDO"},
			Organizations: []partner.Organization{{INN: "7700000002"}}},
		// EDO в 1С не отмечено, но трафик по ИНН есть; связан через ИНН идентификатора без владельца.
		{Code: "CL-3", Subjects: []string{"SUBSCRIPTION"},
			Organizations: []partner.Organization{{INN: "7700000003"}}},
		// Только ИТС: к ЭДО отношения не имеет.
		{Code: "CL-4", Subjects: []string{"SUBSCRIPTION"}},
	}
	ids := []store.IdentifierRecord{
		{EDOID: "A", OwnerCode: "CL-1", INN: "7700000001", LastPeriod: "2026-08"},
		{EDOID: "B", INN: "7700000003", LastPeriod: "2026-08"},
		// Владельца в базе нет, ИНН тоже — «в биллинге, но нет в базе».
		{EDOID: "C", OwnerCode: "CL-9", INN: "7700000009", LastPeriod: "2026-08"},
		// Старый месяц: в сверку «нет в базе» не идёт.
		{EDOID: "D", OwnerCode: "CL-8", INN: "7700000008", LastPeriod: "2026-06"},
	}
	traffic := []partner.EDOTrafficRow{{INN: "7700000003"}}

	check := CheckSubscribers(subs, ids, traffic)
	if check.Period != "2026-08" {
		t.Errorf("Period = %q", check.Period)
	}
	want := map[string]SubscriberCoverage{
		"CL-1": {EDO: true, InBilling: true, EDOIDs: []string{"A"}},
		"CL-2": {EDO: true},
		"CL-3": {EDO: true, InTraffic: true, InBilling: true, EDOIDs: []string{"B"}},
		"CL-4": {},
	}
	for code, w := range want {
		got := check.Coverage[code]
		if got.EDO != w.EDO || got.InTraffic != w.InTraffic || got.InBilling != w.InBilling || !slices.Equal(got.EDOIDs, w.EDOIDs) {
			t.Errorf("%s: %+v, ожидали %+v", code, got, w)
		}
	}
	if len(check.NotInBase) != 1 || !slices.Equal(check.NotInBase[0].EDOIDs, []string{"C"}) {
		t.Errorf("NotInBase = %+v, ожидали только C", check.NotInBase)
	}

	if empty := CheckSubscribers(nil, ids, traffic); len(empty.NotInBase) != 0 {
		t.Errorf("без базы абонентов сверки «нет в базе» быть не должно: %+v", empty.NotInBase)
	}
}

// ИНН из нулей — организация, заведённая по e-mail: чужие идентификаторы с
// таким же ИНН абоненту не достаются, и в биллинге он от них не оказывается.
func TestCheckSubscribersIgnoresZeroINN(t *testing.T) {
	subs := []store.SubscriberRecord{{Code: "CL-5", Subjects: []string{"EDO"},
		Organizations: []partner.Organization{{Name: "mail@example.test", INN: "0000000000"}}}}
	ids := []store.IdentifierRecord{{EDOID: "Z", INN: "0000000000", LastPeriod: "2026-08"}}
	traffic := []partner.EDOTrafficRow{{INN: "0000000000"}}

	check := CheckSubscribers(subs, ids, traffic)
	if got := check.Coverage["CL-5"]; got.InBilling || got.InTraffic || len(got.EDOIDs) != 0 {
		t.Errorf("CL-5 склеен по нулевому ИНН: %+v", got)
	}
	if len(check.NotInBase) != 1 || check.NotInBase[0].EDOIDs[0] != "Z" {
		t.Errorf("NotInBase = %+v, ожидали Z", check.NotInBase)
	}
}

// Обезличенный случай из реальной базы: у организации два идентификатора ЭДО
// с одним ИНН, КПП и логином — один с владельцем, второй без. Владельца 1С
// в базе абонентов не отдаёт; в отчёте трафика колонка «Абонент» — «CL-7: Имя».
func TestCheckSubscribersGroupsIdentifiersOfOneOrganization(t *testing.T) {
	subs := []store.SubscriberRecord{
		{Code: "CL-1", Subjects: []string{"EDO"}},
		{Code: "CL-2", Subjects: []string{"EDO"}},
	}
	ids := []store.IdentifierRecord{
		{EDOID: "A", OwnerCode: "CL-7", INN: "7700000007", KPP: "770001001", Login: "user7", LastPeriod: "2026-08"},
		{EDOID: "B", INN: "7700000007", KPP: "770001001", Login: "user7", LastPeriod: "2026-08"},
		// Владелец в базе, а ИНН его организации в 1С не записан: второй
		// идентификатор того же ИНН без владельца — тоже его.
		{EDOID: "C", OwnerCode: "CL-1", INN: "7700000001", LastPeriod: "2026-08"},
		{EDOID: "D", INN: "7700000001", LastPeriod: "2026-08"},
		// Без владельца и без ИНН в базе, но трафик знает владельца.
		{EDOID: "E", INN: "7700000005", Login: "user5", LastPeriod: "2026-08"},
	}
	traffic := []partner.EDOTrafficRow{
		{Subscriber: "CL-2: Иванов", INN: "7700000002"},
		{Subscriber: "CL-5: Петров", INN: "7700000005"},
	}

	check := CheckSubscribers(subs, ids, traffic)

	if got := check.Coverage["CL-1"].EDOIDs; !slices.Equal(got, []string{"C", "D"}) {
		t.Errorf("CL-1: идентификаторы %v, ожидали [C D]", got)
	}
	if !check.Coverage["CL-2"].InTraffic {
		t.Error("CL-2: трафик по коду из «CL-2: Иванов» не найден")
	}
	if len(check.NotInBase) != 2 {
		t.Fatalf("NotInBase = %+v, ожидали 2 организации", check.NotInBase)
	}
	first := check.NotInBase[0]
	if !slices.Equal(first.EDOIDs, []string{"A", "B"}) || !slices.Equal(first.Logins, []string{"user7"}) ||
		!slices.Equal(first.OwnerCodes, []string{"CL-7"}) {
		t.Errorf("два идентификатора одной организации не собраны: %+v", first)
	}
	if second := check.NotInBase[1]; !slices.Equal(second.OwnerCodes, []string{"CL-5"}) {
		t.Errorf("владелец из отчёта трафика не подставлен: %+v", second)
	}
}

type fakeSubscriberSource struct {
	list  []partner.Subscriber
	calls int
}

func (f *fakeSubscriberSource) AllSubscribers(ctx context.Context) ([]partner.Subscriber, error) {
	f.calls++
	return f.list, nil
}

type fakeSubscriberStore struct {
	saved []partner.Subscriber
	last  time.Time
}

func (f *fakeSubscriberStore) Save(ctx context.Context, list []partner.Subscriber, at time.Time) error {
	f.saved, f.last = list, at
	return nil
}

func (f *fakeSubscriberStore) LastFetched(ctx context.Context) (time.Time, error) { return f.last, nil }

func TestSubscriberRefresherSkipsFreshAndRejectsEmpty(t *testing.T) {
	source := &fakeSubscriberSource{}
	st := &fakeSubscriberStore{}
	refresher := NewSubscriberRefresher(source, st)

	if _, err := refresher.Refresh(t.Context(), SubscribersRefreshInterval); err == nil || st.saved != nil {
		t.Fatalf("пустая выгрузка должна быть ошибкой и не сохраняться: %v", err)
	}
	source.list = []partner.Subscriber{{Code: "CL-1"}}
	if called, err := refresher.Refresh(t.Context(), SubscribersRefreshInterval); err != nil || !called || len(st.saved) != 1 {
		t.Fatalf("выгрузка: called=%v, err=%v", called, err)
	}
	if called, _ := refresher.Refresh(t.Context(), SubscribersRefreshInterval); called || source.calls != 2 {
		t.Errorf("свежая база выгружена повторно: called=%v, вызовов %d", called, source.calls)
	}
}

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"partnerops/internal/partner"
)

// countingSource — API 1С в памяти, считает вызовы. Реквизиты вымышлены.
type countingSource struct {
	calls map[string]int
	fail  bool
}

func (s *countingSource) hit(name string) error {
	s.calls[name]++
	if s.fail {
		return errors.New("1С не ответила")
	}
	return nil
}

func (s *countingSource) Subscribers(context.Context) ([]partner.Subscriber, error) {
	return []partner.Subscriber{{Code: "CL-1000001", RegNumbers: []string{"800000001"},
		Organizations: []partner.Organization{{Name: "ООО Альфа", INN: "7700000001", KPP: "770001001"}}}}, s.hit("subscribers")
}

func (s *countingSource) NomenclatureByRegNumbers(_ context.Context, regs []string) ([]partner.Nomenclature, error) {
	s.calls["nomenclature:"+regs[0]]++
	return []partner.Nomenclature{{RegNumber: "800000001", Name: "Бухгалтерия"}}, s.hit("nomenclature")
}

func (s *countingSource) CheckITSByRegNumbers(context.Context, []string) ([]partner.ITSCheck, error) {
	n := 130
	return []partner.ITSCheck{{RegNumber: "800000001", Contracts: []partner.ITSContract{
		{TypeNameForUser: "ПРОФ", TypeNumber: &n}}}}, s.hit("its")
}

func (s *countingSource) ProgramsByLogin(context.Context, string) ([]partner.ProgramAccess, error) {
	return []partner.ProgramAccess{{RegNumber: "800000001"}}, s.hit("login")
}

func (s *countingSource) UserExists(context.Context, string, string) (bool, error) {
	return true, s.hit("users")
}

func (s *countingSource) RegNumberRegistered(context.Context, string) (bool, error) {
	return true, s.hit("reg")
}

func (s *countingSource) EDOTrafficLogins(context.Context, string, time.Time, time.Time) ([]string, error) {
	return []string{"client"}, s.hit("traffic")
}

func TestRequestDirectoryCachesLookups(t *testing.T) {
	source := &countingSource{calls: map[string]int{}}
	dir := NewRequestDirectory(source)
	now := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	dir.now = func() time.Time { return now }
	ctx := context.Background()

	for range 3 {
		subs, err := dir.Subscribers(ctx)
		if err != nil || len(subs) != 1 || subs[0].Organizations[0].INN != "7700000001" {
			t.Fatalf("Subscribers = %+v, %v", subs, err)
		}
		products, _ := dir.Products(ctx, []string{"800000001", "800000002"})
		if products["800000001"] != "Бухгалтерия" || len(products) != 1 {
			t.Fatalf("Products = %v", products)
		}
		contracts, _ := dir.Contracts(ctx, []string{"800000001"})
		if len(contracts["800000001"]) != 1 || contracts["800000001"][0].Name != "ПРОФ" {
			t.Fatalf("Contracts = %+v", contracts)
		}
		_, _ = dir.LoginRegNumbers(ctx, "Client")
		_, _ = dir.UserExists(ctx, "client", "client@example.ru")
		_, _ = dir.RegNumberRegistered(ctx, "800000001")
		_, _ = dir.EDOTrafficLogins(ctx, "7700000001", now.AddDate(-1, 0, 0), now)
	}
	for _, name := range []string{"subscribers", "nomenclature", "its", "login", "users", "reg", "traffic"} {
		if source.calls[name] != 1 {
			t.Errorf("%s вызван %d раз, ожидали 1", name, source.calls[name])
		}
	}

	// Через lookupTTL точечные справки спрашиваются заново, база абонентов — нет.
	now = now.Add(lookupTTL + time.Minute)
	_, _ = dir.Subscribers(ctx)
	_, _ = dir.LoginRegNumbers(ctx, "client")
	if source.calls["subscribers"] != 1 || source.calls["login"] != 2 {
		t.Errorf("после TTL: %v", source.calls)
	}
}

func TestRequestDirectoryDoesNotCacheErrors(t *testing.T) {
	source := &countingSource{calls: map[string]int{}, fail: true}
	dir := NewRequestDirectory(source)
	now := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	dir.now = func() time.Time { return now }
	ctx := context.Background()
	for range 2 {
		if _, err := dir.Subscribers(ctx); err == nil {
			t.Fatal("ожидали ошибку")
		}
		if _, err := dir.EDOTrafficLogins(ctx, "7700000001", time.Time{}, time.Time{}); err == nil {
			t.Fatal("ожидали ошибку")
		}
	}
	// База абонентов после отказа ждёт subscribersRetry: форма не долбит 1С.
	if source.calls["subscribers"] != 1 || source.calls["traffic"] != 2 {
		t.Errorf("вызовы после отказа: %v", source.calls)
	}
	now = now.Add(subscribersRetry + time.Second)
	source.fail = false
	if subs, err := dir.Subscribers(ctx); err != nil || len(subs) != 1 || source.calls["subscribers"] != 2 {
		t.Errorf("повтор после паузы: %v, %v", err, source.calls)
	}
}

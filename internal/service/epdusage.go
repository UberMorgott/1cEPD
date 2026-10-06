package service

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"partnerops/internal/itsreq"
	"partnerops/internal/partner"
	"partnerops/internal/store"
)

// EPDUsageSource строит отчёт трафика ЭДО; *partner.Client ему удовлетворяет.
type EPDUsageSource interface {
	EDOTrafficReport(ctx context.Context, from, to time.Time) ([]byte, error)
}

// EPDUsageStore хранит последний отчёт.
type EPDUsageStore interface {
	Replace(ctx context.Context, run store.EPDUsageRun, rows []partner.EDOTrafficRow) error
	Run(ctx context.Context) (store.EPDUsageRun, bool, error)
}

// EPDUsageRefresher держит расход ЭПД за последние 12 закрытых месяцев.
type EPDUsageRefresher struct {
	source EPDUsageSource
	store  EPDUsageStore
	now    func() time.Time
}

// NewEPDUsageRefresher собирает сервис.
func NewEPDUsageRefresher(source EPDUsageSource, store EPDUsageStore) *EPDUsageRefresher {
	return &EPDUsageRefresher{source: source, store: store, now: time.Now}
}

// Refresh строит отчёт, только если закрылся новый месяц: закрытые месяцы не
// меняются, а каждый отчёт тратит часовой лимит 1С. Одна попытка за вызов —
// при ошибке повтор будет на следующем суточном прогоне, не циклом.
func (s *EPDUsageRefresher) Refresh(ctx context.Context) (bool, error) {
	from, to := lastTwelveMonths(s.now())
	run := store.EPDUsageRun{PeriodFrom: from.Format("2006-01"), PeriodTo: to.Format("2006-01")}

	saved, ok, err := s.store.Run(ctx)
	if err != nil {
		return false, err
	}
	if ok && saved.PeriodFrom == run.PeriodFrom && saved.PeriodTo == run.PeriodTo {
		return false, nil
	}

	// Конец периода — последняя секунда последнего закрытого месяца.
	end := to.AddDate(0, 1, 0).Add(-time.Second)
	raw, err := s.source.EDOTrafficReport(ctx, from, end)
	if err != nil {
		return true, fmt.Errorf("service: не построить отчёт трафика за %s…%s: %w",
			run.PeriodFrom, run.PeriodTo, err)
	}
	rows, err := partner.ParseEDOTraffic(raw)
	if err != nil {
		return true, fmt.Errorf("service: не разобрать отчёт трафика: %w", err)
	}
	run.FetchedAt = s.now().UTC()
	if err := s.store.Replace(ctx, run, rows); err != nil {
		return true, err
	}
	slog.Info("расход ЭПД обновлён", "from", run.PeriodFrom, "to", run.PeriodTo, "rows", len(rows))
	return true, nil
}

// lastTwelveMonths возвращает первые числа первого и последнего из 12
// закрытых месяцев перед now.
func lastTwelveMonths(now time.Time) (from, to time.Time) {
	year, month, _ := now.UTC().Date()
	current := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	return current.AddDate(0, -12, 0), current.AddDate(0, -1, 0)
}

// EPDAdvice — подсказка тарифа ЭПД для организации.
type EPDAdvice struct {
	INN        string
	KPP        string
	ClientName string
	Subscriber string
	// Fresh — владелец в облаке Фреш: тариф оформляется в Менеджере сервиса.
	Fresh bool
	// EPDOut — исходящие ЭПД за 12 месяцев: как и в биллинге ЭДО, платит
	// отправитель (в биллинге считаются только СФ_исх и не_СФ_исх).
	EPDOut int64
	EPDIn  int64
	// Current — тарифы ЭПД, которые видны у клиента в биллинге.
	Current            []itsreq.EPDHolding
	CurrentCostKopeks  int64
	PerPieceCostKopeks int64
	// Best — выгоднейший тариф; BestOK false — выгоднее поштучно.
	Best           itsreq.Tariff
	BestOK         bool
	BestCostKopeks int64
	SavingsKopeks  int64
	Individual     bool
	AlreadyOptimal bool
}

// BuildEPDAdvice считает подсказки по организациям (ИНН/КПП): годовой расход
// из отчёта трафика против цен тарифов (docs/REQUESTS.md §6). Организации без
// расхода и без тарифа ЭПД пропускаются — подсказывать нечего.
func BuildEPDAdvice(rows []partner.EDOTrafficRow, registry []store.IdentifierRecord) []EPDAdvice {
	byOrg := map[string]*EPDAdvice{}
	var order []string
	org := func(inn, kpp string) *EPDAdvice {
		key := inn + "/" + kpp
		if a, ok := byOrg[key]; ok {
			return a
		}
		a := &EPDAdvice{INN: inn, KPP: kpp}
		byOrg[key] = a
		order = append(order, key)
		return a
	}

	for _, r := range rows {
		inn := strings.TrimSpace(r.INN)
		if inn == "" {
			continue
		}
		a := org(inn, strings.TrimSpace(r.KPP))
		a.EPDOut += r.EPDOut
		a.EPDIn += r.EPDIn
		if a.ClientName == "" {
			a.ClientName = r.ClientName
		}
		if code, _ := partner.ParseOwner(r.Subscriber); code != "" && a.Subscriber == "" {
			a.Subscriber = code
		}
	}

	// Тарифы из биллинга: у двух идентификаторов одной организации строка
	// «Тарифы ИТС» повторяется — берём максимум подписок по коду, а не сумму.
	held := map[string]map[string]itsreq.EPDHolding{}
	for _, record := range registry {
		inn := strings.TrimSpace(record.INN)
		holdings := itsreq.EPDTariffsInText(record.ITSTariffs)
		if inn == "" || len(holdings) == 0 {
			continue
		}
		key := inn + "/" + strings.TrimSpace(record.KPP)
		if held[key] == nil {
			held[key] = map[string]itsreq.EPDHolding{}
		}
		for _, h := range holdings {
			if h.Count > held[key][h.Tariff.Code].Count {
				held[key][h.Tariff.Code] = h
			}
		}
		a := org(inn, strings.TrimSpace(record.KPP))
		if a.ClientName == "" {
			a.ClientName = record.ClientName
		}
		if a.Subscriber == "" {
			a.Subscriber = record.OwnerCode
		}
	}

	list := make([]EPDAdvice, 0, len(order))
	for _, key := range order {
		a := byOrg[key]
		var covered, packages int64
		for _, h := range held[key] {
			a.Current = append(a.Current, h)
			covered += int64(h.Tariff.Volume) * int64(h.Count)
			packages += h.Tariff.RetailKopeks * int64(h.Count)
		}
		if a.EPDOut == 0 && len(a.Current) == 0 {
			continue
		}
		sort.Slice(a.Current, func(i, j int) bool { return a.Current[i].Tariff.Volume < a.Current[j].Tariff.Volume })
		a.Fresh = partner.IsFreshOwner(a.Subscriber)
		a.CurrentCostKopeks = itsreq.YearCostKopeks(a.EPDOut, covered, packages)
		a.PerPieceCostKopeks = itsreq.YearCostKopeks(a.EPDOut, 0, 0)
		a.Best, a.BestOK, a.BestCostKopeks = itsreq.BestEPDTariff(a.EPDOut)
		a.SavingsKopeks = a.CurrentCostKopeks - a.BestCostKopeks
		a.Individual = a.EPDOut >= itsreq.IndividualVolume
		a.AlreadyOptimal = a.SavingsKopeks <= 0
		list = append(list, *a)
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].SavingsKopeks > list[j].SavingsKopeks })
	return list
}

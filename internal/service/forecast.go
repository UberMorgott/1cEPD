package service

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"time"

	"partnerops/internal/partner"
	"partnerops/internal/store"
)

// monthTrafficEvery — не чаще одного отчёта трафика текущего месяца за этот срок:
// перезапуск сервиса не должен тратить лимит отчётов 1С.
const monthTrafficEvery = 20 * time.Hour

// forecastMinDays — раньше экстраполяция по паре дней слишком шумная: до этого
// предупреждаем только о лимите, который уже превышен.
const forecastMinDays = 3

// MonthTrafficRefresher держит отчёт трафика ЭДО за текущий месяц: с первого
// числа до момента построения. Биллинг за текущий месяц 1С не отдаёт
// (BILLING_DOES_NOT_EXIST), а трафик без фильтра клиента — отдаёт.
type MonthTrafficRefresher struct {
	source EPDUsageSource
	store  EPDUsageStore
	now    func() time.Time
}

// NewMonthTrafficRefresher собирает сервис.
func NewMonthTrafficRefresher(source EPDUsageSource, store EPDUsageStore) *MonthTrafficRefresher {
	return &MonthTrafficRefresher{source: source, store: store, now: time.Now}
}

// Refresh строит отчёт раз в сутки. Одна попытка за вызов — повтор на
// следующем суточном прогоне, не циклом.
func (s *MonthTrafficRefresher) Refresh(ctx context.Context) (bool, error) {
	now := s.now().UTC()
	start := monthStart(now)
	period := start.Format("2006-01")

	saved, ok, err := s.store.Run(ctx)
	if err != nil {
		return false, err
	}
	if ok && saved.PeriodFrom == period && now.Sub(saved.FetchedAt) < monthTrafficEvery {
		return false, nil
	}

	raw, err := s.source.EDOTrafficReport(ctx, start, now)
	if err != nil {
		return true, fmt.Errorf("service: не построить отчёт трафика за %s: %w", period, err)
	}
	rows, err := partner.ParseEDOTraffic(raw)
	if err != nil {
		return true, fmt.Errorf("service: не разобрать отчёт трафика за %s: %w", period, err)
	}
	run := store.EPDUsageRun{PeriodFrom: period, PeriodTo: period, FetchedAt: now}
	if err := s.store.Replace(ctx, run, rows); err != nil {
		return true, err
	}
	slog.Info("трафик текущего месяца обновлён", "period", period, "rows", len(rows))
	return true, nil
}

func monthStart(t time.Time) time.Time {
	year, month, _ := t.UTC().Date()
	return time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
}

// Forecast — прогноз расхода лимита идентификатора к концу месяца.
type Forecast struct {
	EDOID      string `json:"edoId"`
	ClientName string `json:"clientName"`
	INN        string `json:"inn"`
	KPP        string `json:"kpp"`
	Login      string `json:"login"`
	Limit      int64  `json:"limit"`
	// Used — исходящие СФ с начала месяца: оценка пакетов биллинга.
	Used      int64 `json:"used"`
	Projected int64 `json:"projected"`
	// OverBy — на сколько прогноз больше лимита; 0 — укладывается.
	OverBy   int64 `json:"overBy"`
	Exceeded bool  `json:"exceeded"`
	// Reliable — данных достаточно для экстраполяции (прошло forecastMinDays).
	Reliable bool `json:"reliable"`
	Warn     bool `json:"warn"`
}

// MonthForecast — прогноз по всем идентификаторам с лимитом.
type MonthForecast struct {
	Period      string     `json:"period"`
	AsOf        time.Time  `json:"asOf"`
	DaysElapsed float64    `json:"daysElapsed"`
	DaysInMonth int        `json:"daysInMonth"`
	Clients     []Forecast `json:"clients"`
}

// BuildForecast экстраполирует расход с начала месяца на весь месяц и сравнивает
// с лимитом из реестра (последний снимок биллинга). Расход растёт линейно по
// времени — грубо, но честно: подсказка «присмотреться», а не счёт.
func BuildForecast(run store.EPDUsageRun, rows []partner.EDOTrafficRow, registry []store.IdentifierRecord) MonthForecast {
	start, err := time.Parse("2006-01", run.PeriodFrom)
	if err != nil {
		return MonthForecast{Period: run.PeriodFrom, Clients: []Forecast{}}
	}
	end := start.AddDate(0, 1, 0)
	elapsed := min(run.FetchedAt.Sub(start), end.Sub(start))
	result := MonthForecast{
		Period: run.PeriodFrom, AsOf: run.FetchedAt,
		DaysElapsed: elapsed.Hours() / 24, DaysInMonth: int(end.Sub(start).Hours() / 24),
		Clients: []Forecast{},
	}

	used := map[string]int64{}
	for _, r := range rows {
		used[r.EDOID] += r.InvoicesOut
	}
	for _, record := range registry {
		spent := used[record.EDOID]
		if record.Limit == nil || *record.Limit <= 0 || spent == 0 {
			continue
		}
		f := Forecast{
			EDOID: record.EDOID, ClientName: record.ClientName, INN: record.INN, KPP: record.KPP,
			Login: record.Login, Limit: *record.Limit, Used: spent, Projected: spent,
			Exceeded: spent > *record.Limit,
			Reliable: result.DaysElapsed >= forecastMinDays,
		}
		if elapsed > 0 {
			total := float64(end.Sub(start))
			f.Projected = int64(math.Ceil(float64(spent) * total / float64(elapsed)))
		}
		f.OverBy = max(0, f.Projected-f.Limit)
		f.Warn = f.Exceeded || (f.Reliable && f.OverBy > 0)
		result.Clients = append(result.Clients, f)
	}
	sort.SliceStable(result.Clients, func(i, j int) bool {
		a, b := result.Clients[i], result.Clients[j]
		if a.Warn != b.Warn {
			return a.Warn
		}
		return a.OverBy > b.OverBy
	})
	return result
}

package service

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"partnerops/internal/partner"
	"partnerops/internal/store"
)

// OptionsRefreshInterval — отчёт по опциям моложе этого не перестраивается.
// Чуть меньше суток, как у договоров 1С:ИТС: тик, пришедший на минуту раньше,
// не должен пропускать обновление ещё на сутки.
const OptionsRefreshInterval = 20 * time.Hour

// OptionsRateLimitPause — пауза после отказа 1С по часовому лимиту задач.
const OptionsRateLimitPause = time.Hour

// OptionsSource строит отчёт по опциям; *partner.Client ему удовлетворяет.
type OptionsSource interface {
	OptionBillingReport(ctx context.Context, reportType string) (partner.OptionReport, error)
}

// OptionsStore хранит отчёты по опциям.
type OptionsStore interface {
	Save(ctx context.Context, report store.OptionReport) error
	Fetched(ctx context.Context) (map[string]time.Time, error)
}

// OptionsRefresher держит свежими остатки и сроки лицензий сервисов.
type OptionsRefresher struct {
	source OptionsSource
	store  OptionsStore
	now    func() time.Time
	// pausedUntil — до этого момента 1С не беспокоим: упёрлись в часовой лимит.
	pausedUntil time.Time
}

// NewOptionsRefresher собирает сервис.
func NewOptionsRefresher(source OptionsSource, store OptionsStore) *OptionsRefresher {
	return &OptionsRefresher{source: source, store: store, now: time.Now}
}

// Step строит не больше одного отчёта: первый по порядку вид, у которого нет
// отчёта моложе OptionsRefreshInterval. Видов одиннадцать, и каждый тратит
// часовой лимит задач 1С, который нужен и биллингу, — поэтому по одному за шаг,
// а не все разом. Возвращает, ходили ли в 1С.
func (s *OptionsRefresher) Step(ctx context.Context) (bool, error) {
	now := s.now().UTC()
	if now.Before(s.pausedUntil) {
		return false, nil
	}
	fetched, err := s.store.Fetched(ctx)
	if err != nil {
		return false, err
	}
	kind := ""
	for _, candidate := range partner.OptionReportTypes {
		if last, ok := fetched[candidate]; !ok || now.Sub(last) >= OptionsRefreshInterval {
			kind = candidate
			break
		}
	}
	if kind == "" {
		return false, nil
	}

	report, err := s.source.OptionBillingReport(ctx, kind)
	switch {
	case partner.IsNoBilling(err):
		// Предметный отказ: тарифов этого вида нет, повтор через сутки.
		return true, s.store.Save(ctx, store.OptionReport{Type: kind, State: store.OptionStateNone, FetchedAt: now})
	case partner.IsRateLimited(err):
		s.pausedUntil = now.Add(OptionsRateLimitPause)
		return true, fmt.Errorf("service: отчёт по опциям %s отложен на час: %w", kind, err)
	case err != nil:
		// Прочие ошибки: вид остаётся несвежим, следующий шаг попробует снова.
		// Чтобы не молотить 1С, пауза та же, что и после лимита.
		s.pausedUntil = now.Add(OptionsRateLimitPause)
		return true, fmt.Errorf("service: не построить отчёт по опциям %s: %w", kind, err)
	}
	if err := s.store.Save(ctx, store.OptionReport{Type: kind, State: store.OptionStateOK,
		Entries: report.Entries, FetchedAt: now}); err != nil {
		return true, err
	}
	slog.Info("отчёт по опциям обновлён", "type", kind, "subscribers", len(report.Entries))
	return true, nil
}

// LicenseOption — опция тарифа с видом отчёта, из которого она пришла.
type LicenseOption struct {
	Type string
	partner.Option
}

// Remaining — остаток количественной опции; ok false — объёмов нет.
func (o LicenseOption) Remaining() (float64, bool) {
	if !o.Quantitative || o.MaxVolume == nil || o.UsedVolume == nil {
		return 0, false
	}
	return *o.MaxVolume - *o.UsedVolume, true
}

// LicenseTariff — тариф абонента с опциями всех видов. Один тариф (например,
// ИТСааС ПРОФ) приходит в отчётах нескольких видов — здесь он один.
type LicenseTariff struct {
	SubscriberCode string
	partner.OptionTariff
	Options []LicenseOption
}

// LicenseTariffs сводит отчёты всех видов в тарифы по коду абонента.
func LicenseTariffs(reports []store.OptionReport) map[string][]LicenseTariff {
	result := map[string][]LicenseTariff{}
	index := map[string]int{}
	for _, report := range reports {
		for _, entry := range report.Entries {
			for _, t := range entry.Tariffs {
				key := entry.SubscriberCode + "|" + t.Name + "|" + t.OrgINN + "|" + t.OrgKPP + "|" +
					t.Start.Format(time.RFC3339) + "|" + t.End.Format(time.RFC3339)
				at, ok := index[key]
				if !ok {
					plain := t
					plain.Options = nil
					at = len(result[entry.SubscriberCode])
					index[key] = at
					result[entry.SubscriberCode] = append(result[entry.SubscriberCode],
						LicenseTariff{SubscriberCode: entry.SubscriberCode, OptionTariff: plain})
				}
				tariff := &result[entry.SubscriberCode][at]
				for _, o := range t.Options {
					tariff.Options = append(tariff.Options, LicenseOption{Type: report.Type, Option: o})
				}
			}
		}
	}
	for code := range result {
		sort.SliceStable(result[code], func(i, j int) bool {
			return result[code][i].End.Before(result[code][j].End)
		})
	}
	return result
}

// ExpiringLicense — тариф сервиса, который скоро кончится.
type ExpiringLicense struct {
	Tariff   LicenseTariff
	DaysLeft int
}

// ExpiringLicenses отбирает тарифы, которые кончаются не позже чем через days
// суток, — по тем же правилам, что ExpiringContracts: продлённым тариф считается,
// если у той же организации абонента есть тариф с тем же названием и более
// поздним окончанием; истёкшие раньше окна назад не показываются.
func ExpiringLicenses(tariffs map[string][]LicenseTariff, now time.Time, days int) []ExpiringLicense {
	horizon := now.Add(time.Duration(days) * 24 * time.Hour)
	oldest := now.Add(-time.Duration(days) * 24 * time.Hour)

	var list []ExpiringLicense
	for _, owned := range tariffs {
		renewed := renewedTariffs(owned)
		for i, t := range owned {
			if t.End.IsZero() || t.End.After(horizon) || t.End.Before(oldest) || renewed[i] {
				continue
			}
			list = append(list, ExpiringLicense{Tariff: t, DaysLeft: int(t.End.Sub(now).Hours() / 24)})
		}
	}
	sort.SliceStable(list, func(i, j int) bool {
		if !list[i].Tariff.End.Equal(list[j].Tariff.End) {
			return list[i].Tariff.End.Before(list[j].Tariff.End)
		}
		return list[i].Tariff.SubscriberCode < list[j].Tariff.SubscriberCode
	})
	return list
}

// renewedTariffs отмечает тарифы, у которых у той же организации абонента есть
// тариф с тем же названием и более поздним окончанием: продолжение уже оформлено.
func renewedTariffs(owned []LicenseTariff) []bool {
	key := func(t LicenseTariff) string { return t.Name + "|" + t.OrgINN + "|" + t.OrgKPP }
	latest := map[string]time.Time{}
	for _, t := range owned {
		if t.End.After(latest[key(t)]) {
			latest[key(t)] = t.End
		}
	}
	renewed := make([]bool, len(owned))
	for i, t := range owned {
		renewed[i] = latest[key(t)].After(t.End)
	}
	return renewed
}

// licenseLowMinVolume — опции меньшего объёма в «мало остатка» не попадают:
// лицензия «1 из 1» на одну организацию — обычное состояние, а не повод звонить.
// Перерасход (израсходовано больше объёма) показывается при любом объёме.
const licenseLowMinVolume = 2

// LowLicense — опция действующего тарифа, у которой почти кончился объём.
type LowLicense struct {
	Tariff    LicenseTariff
	Option    LicenseOption
	Remaining float64
	Over      bool
}

// LowLicenses отбирает количественные опции действующих непродлённых тарифов с
// остатком не больше 10 % объёма — тот же порог, что у лимита ЭДО в биллинге.
func LowLicenses(tariffs map[string][]LicenseTariff, now time.Time) []LowLicense {
	var list []LowLicense
	for _, owned := range tariffs {
		renewed := renewedTariffs(owned)
		for i, t := range owned {
			if renewed[i] || !t.End.IsZero() && t.End.Before(now) {
				continue
			}
			for _, o := range t.Options {
				remaining, ok := o.Remaining()
				if !ok || *o.MaxVolume <= 0 {
					continue
				}
				over := remaining < 0
				low := *o.MaxVolume >= licenseLowMinVolume && remaining <= *o.MaxVolume*lowRemainderShare
				if over || low {
					list = append(list, LowLicense{Tariff: t, Option: o, Remaining: remaining, Over: over})
				}
			}
		}
	}
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].Over != list[j].Over {
			return list[i].Over
		}
		if list[i].Remaining != list[j].Remaining {
			return list[i].Remaining < list[j].Remaining
		}
		return list[i].Tariff.SubscriberCode < list[j].Tariff.SubscriberCode
	})
	return list
}

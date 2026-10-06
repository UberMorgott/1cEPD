package service

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"partnerops/internal/partner"
	"partnerops/internal/store"
)

// ITSRefreshInterval — проверка моложе этого не повторяется суточной задачей.
// Сроки договоров меняются редко, а сервис 1С стоит за DDoS-Guard (docs/API.md
// §12, грабля 10). Чуть меньше суток: иначе тик, пришедший на минуту раньше,
// пропустил бы проверку ещё на сутки.
const ITSRefreshInterval = 20 * time.Hour

// ITSSource проверяет договоры в 1С; *partner.Client ему удовлетворяет.
type ITSSource interface {
	CheckITSBySubscriberCodes(ctx context.Context, codes []string) ([]partner.ITSCheck, error)
}

// ITSStore хранит последнюю проверку.
type ITSStore interface {
	Replace(ctx context.Context, checks []partner.ITSCheck, at time.Time) error
	LastChecked(ctx context.Context) (time.Time, error)
}

// OwnerSource — реестр идентификаторов: из него берутся коды абонентов.
type OwnerSource interface {
	All(ctx context.Context) ([]store.IdentifierRecord, error)
}

// IndustrySource проверяет ИТС Отраслевой в 1С; *partner.Client ему удовлетворяет.
type IndustrySource interface {
	CheckIndustryBySubscriberCodes(ctx context.Context, codes []string) ([]partner.IndustryCheck, error)
}

// IndustryStore хранит последнюю проверку ИТС Отраслевого.
type IndustryStore interface {
	Replace(ctx context.Context, checks []partner.IndustryCheck, at time.Time) error
	LastChecked(ctx context.Context) (time.Time, error)
}

// ITSRefresher обновляет сроки договоров 1С:ИТС клиентов из реестра, а с
// WithIndustry — и нужду в ИТС Отраслевом: те же коды абонентов, тот же ритм.
type ITSRefresher struct {
	source         ITSSource
	store          ITSStore
	owners         OwnerSource
	industrySource IndustrySource
	industryStore  IndustryStore
	now            func() time.Time
}

// WithIndustry включает проверку ИТС Отраслевого в тот же прогон.
func (s *ITSRefresher) WithIndustry(source IndustrySource, store IndustryStore) *ITSRefresher {
	s.industrySource, s.industryStore = source, store
	return s
}

// NewITSRefresher собирает сервис.
func NewITSRefresher(source ITSSource, store ITSStore, owners OwnerSource) *ITSRefresher {
	return &ITSRefresher{source: source, store: store, owners: owners, now: time.Now}
}

// ITSManualInterval — не чаще этого проверка повторяется по кнопке.
const ITSManualInterval = 10 * time.Minute

// Refresh проверяет договоры всех владельцев из реестра. Проверку моложе
// maxAge не повторяет: перезапуск сервиса и нетерпеливая кнопка не должны
// гонять запросы в 1С. Возвращает, ходили ли в 1С.
func (s *ITSRefresher) Refresh(ctx context.Context, maxAge time.Duration) (bool, error) {
	now := s.now().UTC()
	last, err := s.store.LastChecked(ctx)
	if err != nil {
		return false, err
	}
	if s.industryStore != nil {
		// Прогон считается свежим, только если свежи обе проверки.
		industry, err := s.industryStore.LastChecked(ctx)
		if err != nil {
			return false, err
		}
		if industry.Before(last) {
			last = industry
		}
	}
	if !last.IsZero() && now.Sub(last) < maxAge {
		return false, nil
	}

	codes, err := OwnerCodes(ctx, s.owners)
	if err != nil {
		return false, err
	}
	if len(codes) == 0 {
		return false, nil
	}
	checks, err := s.source.CheckITSBySubscriberCodes(ctx, codes)
	if err != nil {
		return true, fmt.Errorf("service: не проверить договоры 1С:ИТС: %w", err)
	}
	if err := s.store.Replace(ctx, checks, now); err != nil {
		return true, err
	}
	slog.Info("договоры 1С:ИТС проверены", "subscribers", len(checks))

	if s.industrySource != nil && s.industryStore != nil {
		industry, err := s.industrySource.CheckIndustryBySubscriberCodes(ctx, codes)
		if err != nil {
			return true, fmt.Errorf("service: не проверить ИТС Отраслевой: %w", err)
		}
		if err := s.industryStore.Replace(ctx, industry, now); err != nil {
			return true, err
		}
		slog.Info("ИТС Отраслевой проверен", "subscribers", len(industry))
	}
	return true, nil
}

// OwnerCodes возвращает коды абонентов-владельцев из реестра без повторов.
func OwnerCodes(ctx context.Context, owners OwnerSource) ([]string, error) {
	records, err := owners.All(ctx)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var codes []string
	for _, record := range records {
		code := strings.TrimSpace(record.OwnerCode)
		if code == "" || seen[code] {
			continue
		}
		seen[code] = true
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes, nil
}

// ExpiringContract — договор, срок которого кончается в пределах окна.
type ExpiringContract struct {
	SubscriberCode string
	Contract       partner.ITSContract
	// DaysLeft — полных суток до конца; отрицательное — уже истёк.
	DaysLeft int
}

// ExpiringContracts отбирает договоры, которые кончаются не позже чем через
// days суток от now и ещё не продлены.
//
// Продлённым договор считается, если у того же абонента есть договор того же
// вида (itsContractType.name) с более поздним окончанием: 1С отдаёт и
// действующий договор, и уже оформленное продолжение, — живые данные это
// показали. Давно истёкшие (раньше окна назад) не показываем: это уже не
// напоминание о продлении.
func ExpiringContracts(checks []store.ITSCheck, now time.Time, days int) []ExpiringContract {
	horizon := now.Add(time.Duration(days) * 24 * time.Hour)
	oldest := now.Add(-time.Duration(days) * 24 * time.Hour)

	var list []ExpiringContract
	for _, check := range checks {
		latest := map[string]time.Time{}
		for _, c := range check.Contracts {
			if c.End.After(latest[c.TypeName]) {
				latest[c.TypeName] = c.End
			}
		}
		for _, c := range check.Contracts {
			if c.End.IsZero() || c.End.After(horizon) || c.End.Before(oldest) {
				continue
			}
			if latest[c.TypeName].After(c.End) {
				continue // продолжение уже оформлено
			}
			list = append(list, ExpiringContract{
				SubscriberCode: check.SubscriberCode, Contract: c,
				DaysLeft: int(c.End.Sub(now).Hours() / 24),
			})
		}
	}
	sort.SliceStable(list, func(i, j int) bool {
		return list[i].Contract.End.Before(list[j].Contract.End)
	})
	return list
}

package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"partnerops/internal/partner"
	"partnerops/internal/store"
)

// SubscribersRefreshInterval — выгрузка базы абонентов моложе этого суточной
// задачей не повторяется. Чуть меньше суток, как у ITSRefreshInterval.
const SubscribersRefreshInterval = 20 * time.Hour

// SubscribersManualInterval — не чаще этого выгрузка повторяется по кнопке.
const SubscribersManualInterval = 10 * time.Minute

// SubjectEDO — направление «трафик по 1С-ЭДО» в subjects абонента (docs/API.md §7).
const SubjectEDO = "EDO"

// SubscriberSource выгружает базу абонентов; *RequestDirectory ему
// удовлетворяет — через тот же кэш, что и проверка заявки.
type SubscriberSource interface {
	AllSubscribers(ctx context.Context) ([]partner.Subscriber, error)
}

// SubscriberStore хранит базу абонентов.
type SubscriberStore interface {
	Save(ctx context.Context, list []partner.Subscriber, at time.Time) error
	LastFetched(ctx context.Context) (time.Time, error)
}

// SubscriberRefresher раз в сутки сохраняет базу абонентов партнёра.
type SubscriberRefresher struct {
	source SubscriberSource
	store  SubscriberStore
	now    func() time.Time
}

// NewSubscriberRefresher собирает сервис.
func NewSubscriberRefresher(source SubscriberSource, store SubscriberStore) *SubscriberRefresher {
	return &SubscriberRefresher{source: source, store: store, now: time.Now}
}

// Refresh выгружает базу, если сохранённая старше maxAge. Возвращает,
// выгружалась ли база.
func (s *SubscriberRefresher) Refresh(ctx context.Context, maxAge time.Duration) (bool, error) {
	now := s.now().UTC()
	last, err := s.store.LastFetched(ctx)
	if err != nil {
		return false, err
	}
	if !last.IsZero() && now.Sub(last) < maxAge {
		return false, nil
	}
	list, err := s.source.AllSubscribers(ctx)
	if err != nil {
		return true, fmt.Errorf("service: не выгрузить базу абонентов: %w", err)
	}
	// Пустая выгрузка — сбой: иначе вся база числилась бы пропавшей из 1С.
	if len(list) == 0 {
		return true, errors.New("service: 1С отдала пустую базу абонентов, она не сохранена")
	}
	if err := s.store.Save(ctx, list, now); err != nil {
		return true, err
	}
	slog.Info("база абонентов сохранена", "subscribers", len(list))
	return true, nil
}

// SubscriberCoverage — что об абоненте знают отчёты ЭДО.
type SubscriberCoverage struct {
	// EDO — абонент работает в 1С-ЭДО: направление EDO в 1С или трафик в отчёте.
	EDO bool
	// InTraffic — есть в сохранённом отчёте трафика ЭДО (по коду или ИНН).
	InTraffic bool
	// InBilling — есть в биллинге ЭДО за последний снятый месяц.
	InBilling bool
	// EDOIDs — идентификаторы ЭДО реестра, связанные с абонентом.
	EDOIDs []string
}

// SubscriberCheck — сверка базы абонентов с отчётами ЭДО.
type SubscriberCheck struct {
	// Period — последний месяц биллинга в реестре; пусто — снимков не было.
	Period   string
	Coverage map[string]SubscriberCoverage
	// NotInBase — клиенты из биллинга за Period, которых нет в базе абонентов
	// ни по коду владельца, ни по ИНН. Идентификаторы одной организации
	// (ИНН+КПП) собраны в одну запись.
	NotInBase []NotInBaseClient
}

// NotInBaseClient — организация из биллинга, не найденная в базе абонентов.
type NotInBaseClient struct {
	ClientName string
	INN        string
	KPP        string
	Logins     []string
	// OwnerCodes — коды владельцев из биллинга, а если там пусто — из отчёта
	// трафика по тому же ИНН.
	OwnerCodes []string
	EDOIDs     []string
}

// CheckSubscribers сверяет базу абонентов с реестром ЭДО и отчётом трафика.
//
// Абонент связан с идентификатором по коду владельца или по ИНН одной из своих
// организаций: у идентификатора без владельца (сигнал orphan_*) остаётся только
// ИНН. Идентификатор, найденный по коду владельца, тянет за собой остальные
// идентификаторы своего ИНН: у одной организации бывает второй идентификатор
// без владельца. Этим закрывается разрыв из спеки §4.2 — организации, которые
// видны в кабинете, но не попали в биллинг, и наоборот.
func CheckSubscribers(
	subs []store.SubscriberRecord, ids []store.IdentifierRecord, traffic []partner.EDOTrafficRow,
) SubscriberCheck {
	check := SubscriberCheck{Coverage: map[string]SubscriberCoverage{}}
	for _, id := range ids {
		if id.LastPeriod > check.Period {
			check.Period = id.LastPeriod
		}
	}

	// В отчёте трафика колонка «Абонент» — «CL-1: Имя»: ключом служит код.
	trafficKeys := map[string]bool{}
	trafficOwner := map[string]string{}
	for _, row := range traffic {
		code, _ := partner.ParseOwner(row.Subscriber)
		inn := strings.TrimSpace(row.INN)
		for _, key := range []string{code, inn} {
			if key != "" {
				trafficKeys[key] = true
			}
		}
		if code != "" && inn != "" && trafficOwner[inn] == "" {
			trafficOwner[inn] = code
		}
	}

	byCode := map[string][]store.IdentifierRecord{}
	byINN := map[string][]store.IdentifierRecord{}
	for _, id := range ids {
		if code := strings.TrimSpace(id.OwnerCode); code != "" {
			byCode[code] = append(byCode[code], id)
		}
		// ИНН из нулей 1С ставит организациям, заведённым по одному e-mail:
		// по нему чужие идентификаторы склеились бы с чужим абонентом.
		if inn := strings.TrimSpace(id.INN); realINN(inn) {
			byINN[inn] = append(byINN[inn], id)
		}
	}

	matched := map[string]bool{}
	for _, sub := range subs {
		cover := SubscriberCoverage{
			EDO:       slices.Contains(sub.Subjects, SubjectEDO),
			InTraffic: trafficKeys[sub.Code],
		}
		var linked []store.IdentifierRecord
		for _, id := range byCode[sub.Code] {
			linked = append(linked, id)
			linked = append(linked, byINN[strings.TrimSpace(id.INN)]...)
		}
		for _, org := range sub.Organizations {
			if !realINN(org.INN) {
				continue
			}
			cover.InTraffic = cover.InTraffic || trafficKeys[org.INN]
			linked = append(linked, byINN[org.INN]...)
		}
		for _, id := range linked {
			matched[id.EDOID] = true
			if !slices.Contains(cover.EDOIDs, id.EDOID) {
				cover.EDOIDs = append(cover.EDOIDs, id.EDOID)
			}
			if id.LastPeriod == check.Period {
				cover.InBilling = true
			}
		}
		cover.EDO = cover.EDO || cover.InTraffic
		check.Coverage[sub.Code] = cover
	}

	// Без базы абонентов «нет в базе» были бы все — такую сверку не показываем.
	if len(subs) == 0 {
		return check
	}
	index := map[string]int{}
	for _, id := range ids {
		if id.LastPeriod != check.Period || matched[id.EDOID] {
			continue
		}
		inn, kpp := strings.TrimSpace(id.INN), strings.TrimSpace(id.KPP)
		key := inn + "/" + kpp
		if !realINN(inn) {
			key = "id:" + id.EDOID
		}
		at, ok := index[key]
		if !ok {
			at = len(check.NotInBase)
			index[key] = at
			check.NotInBase = append(check.NotInBase, NotInBaseClient{INN: inn, KPP: kpp})
		}
		client := &check.NotInBase[at]
		if client.ClientName == "" {
			client.ClientName = strings.TrimSpace(id.ClientName)
		}
		client.EDOIDs = appendUnique(client.EDOIDs, id.EDOID)
		client.Logins = appendUnique(client.Logins, strings.TrimSpace(id.Login))
		client.OwnerCodes = appendUnique(client.OwnerCodes, strings.TrimSpace(id.OwnerCode))
	}
	for i := range check.NotInBase {
		client := &check.NotInBase[i]
		if len(client.OwnerCodes) == 0 {
			client.OwnerCodes = appendUnique(client.OwnerCodes, trafficOwner[client.INN])
		}
	}
	return check
}

// appendUnique добавляет непустое значение, которого ещё нет в списке.
func appendUnique(list []string, value string) []string {
	if value == "" || slices.Contains(list, value) {
		return list
	}
	return append(list, value)
}

// realINN — ИНН заполнен и не из одних нулей.
func realINN(inn string) bool {
	return strings.Trim(strings.TrimSpace(inn), "0") != ""
}

package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"partnerops/internal/partner"
	"partnerops/internal/service"
	"partnerops/internal/store"
)

// SubscribersReader — сохранённая база абонентов.
type SubscribersReader interface {
	All(ctx context.Context) ([]store.SubscriberRecord, error)
	LastFetched(ctx context.Context) (time.Time, error)
}

// WithSubscribers включает справочник абонентов. refresher nil — кнопка
// выгрузки недоступна.
func (h *Registry) WithSubscribers(subscribers SubscribersReader, refresher ITSRefresher) *Registry {
	h.subscribers, h.subscribersRefresher = subscribers, refresher
	return h
}

type subscriberOrgItem struct {
	Name string `json:"name"`
	INN  string `json:"inn"`
	KPP  string `json:"kpp"`
	// Key — ключ карточки клиента (см. clientKey).
	Key string `json:"key"`
}

type subscriberItem struct {
	Code          string              `json:"code"`
	Name          string              `json:"name"`
	Subjects      []string            `json:"subjects"`
	RegNumbers    []string            `json:"regNumbers"`
	Organizations []subscriberOrgItem `json:"organizations"`
	FirstSeen     string              `json:"firstSeen"`
	LastSeen      string              `json:"lastSeen"`
	// Gone — в последней выгрузке 1С абонента уже не было.
	Gone      bool     `json:"gone"`
	EDO       bool     `json:"edo"`
	InTraffic bool     `json:"inTraffic"`
	InBilling bool     `json:"inBilling"`
	EDOIDs    []string `json:"edoIds"`
}

type notInBaseItem struct {
	ClientName string   `json:"clientName"`
	INN        string   `json:"inn"`
	KPP        string   `json:"kpp"`
	Logins     []string `json:"logins"`
	OwnerCodes []string `json:"ownerCodes"`
	EDOIDs     []string `json:"edoIds"`
}

// Subscribers отдаёт базу абонентов со сверкой по отчётам ЭДО.
func (h *Registry) Subscribers(w http.ResponseWriter, r *http.Request) {
	if h.subscribers == nil {
		writeError(w, http.StatusServiceUnavailable, "not_configured", "База абонентов не настроена.")
		return
	}
	ctx := r.Context()
	fetchedAt, err := h.subscribers.LastFetched(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось прочитать базу абонентов.")
		return
	}
	subs, err := h.subscribers.All(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось прочитать базу абонентов.")
		return
	}
	ids, err := h.registry.All(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось прочитать реестр.")
		return
	}
	traffic, err := h.trafficRows(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось прочитать отчёт трафика.")
		return
	}

	check := service.CheckSubscribers(subs, ids, traffic)
	list := make([]subscriberItem, 0, len(subs))
	for _, sub := range subs {
		cover := check.Coverage[sub.Code]
		item := subscriberItem{
			Code: sub.Code, Name: sub.Name, Subjects: nonNil(sub.Subjects), RegNumbers: nonNil(sub.RegNumbers),
			Organizations: []subscriberOrgItem{},
			FirstSeen:     sub.FirstSeenAt.Format(time.RFC3339), LastSeen: sub.LastSeenAt.Format(time.RFC3339),
			Gone: sub.LastSeenAt.Before(fetchedAt),
			EDO:  cover.EDO, InTraffic: cover.InTraffic, InBilling: cover.InBilling, EDOIDs: nonNil(cover.EDOIDs),
		}
		for _, org := range sub.Organizations {
			key := clientKey(org.INN, org.KPP)
			if key == "" {
				key = orphanOrgKey(sub.Code, org.Name, org.INN, org.KPP)
			}
			item.Organizations = append(item.Organizations,
				subscriberOrgItem{Name: org.Name, INN: org.INN, KPP: org.KPP, Key: key})
		}
		list = append(list, item)
	}
	notInBase := make([]notInBaseItem, 0, len(check.NotInBase))
	for _, client := range check.NotInBase {
		notInBase = append(notInBase, notInBaseItem{ClientName: client.ClientName, INN: client.INN, KPP: client.KPP,
			Logins: nonNil(client.Logins), OwnerCodes: nonNil(client.OwnerCodes), EDOIDs: nonNil(client.EDOIDs)})
	}

	response := map[string]any{
		"period": check.Period, "subscribers": list, "billingNotInBase": notInBase,
	}
	if !fetchedAt.IsZero() {
		response["fetchedAt"] = fetchedAt.Format(time.RFC3339)
	}
	writeJSON(w, http.StatusOK, response)
}

// RefreshSubscribers выгружает базу абонентов из 1С сейчас. Выгрузку моложе
// service.SubscribersManualInterval не повторяет.
func (h *Registry) RefreshSubscribers(w http.ResponseWriter, r *http.Request) {
	if h.subscribersRefresher == nil {
		writeError(w, http.StatusServiceUnavailable, "not_configured", "Выгрузка базы абонентов не настроена.")
		return
	}
	called, err := h.subscribersRefresher.Refresh(r.Context(), service.SubscribersManualInterval)
	if err != nil {
		slog.Warn("база абонентов не выгружена", "err", err)
		message := "1С не отдала базу абонентов. Попробуйте позже."
		if partner.IsUnauthorized(err) {
			message = "1С не приняла логин и пароль партнёрского API."
		}
		writeError(w, http.StatusBadGateway, "portal_failed", message)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true, "fetched": called})
}

// trafficRows — строки сохранённых отчётов трафика: за год и за текущий месяц.
func (h *Registry) trafficRows(ctx context.Context) ([]partner.EDOTrafficRow, error) {
	var rows []partner.EDOTrafficRow
	for _, source := range []EPDUsageReader{h.epdUsage, h.monthTraffic} {
		if source == nil {
			continue
		}
		list, err := source.Rows(ctx)
		if err != nil {
			return nil, err
		}
		rows = append(rows, list...)
	}
	return rows, nil
}

func nonNil[T any](list []T) []T {
	if list == nil {
		return []T{}
	}
	return list
}

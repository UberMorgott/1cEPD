package httpapi

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"time"

	"partnerops/internal/service"
	"partnerops/internal/store"
)

// OptionReportsStore — сохранённые отчёты по опциям сервисов.
type OptionReportsStore interface {
	All(ctx context.Context) ([]store.OptionReport, error)
}

// WithLicenses включает остатки и сроки лицензий сервисов.
func (h *Registry) WithLicenses(reports OptionReportsStore) *Registry {
	h.optionReports = reports
	return h
}

type licenseOptionItem struct {
	Type         string   `json:"type"`
	Name         string   `json:"name"`
	Quantitative bool     `json:"quantitative"`
	Max          *float64 `json:"max"`
	Used         *float64 `json:"used"`
	Remaining    *float64 `json:"remaining"`
}

type licenseTariffItem struct {
	Name       string              `json:"name"`
	TypeNumber *int                `json:"typeNumber"`
	OrgName    string              `json:"orgName"`
	OrgINN     string              `json:"orgInn"`
	OrgKPP     string              `json:"orgKpp"`
	Start      string              `json:"start"`
	End        string              `json:"end"`
	Services   []string            `json:"services"`
	Options    []licenseOptionItem `json:"options"`
}

type licenseSubscriberItem struct {
	Code    string              `json:"code"`
	Tariffs []licenseTariffItem `json:"tariffs"`
}

type licenseExpiringItem struct {
	SubscriberCode string            `json:"subscriberCode"`
	Tariff         licenseTariffItem `json:"tariff"`
	DaysLeft       int               `json:"daysLeft"`
	Clients        []itsClientItem   `json:"clients"`
}

type licenseLowItem struct {
	SubscriberCode string            `json:"subscriberCode"`
	TariffName     string            `json:"tariffName"`
	OrgName        string            `json:"orgName"`
	End            string            `json:"end"`
	Option         licenseOptionItem `json:"option"`
	Over           bool              `json:"over"`
	Clients        []itsClientItem   `json:"clients"`
}

type licenseReportItem struct {
	Type      string `json:"type"`
	State     string `json:"state"`
	FetchedAt string `json:"fetchedAt"`
}

// Licenses отдаёт тарифы сервисов (1С-Отчетность, 1С:Подпись…) по абонентам,
// тех, у кого тариф кончается в ближайшие days суток, и опции с малым остатком.
func (h *Registry) Licenses(w http.ResponseWriter, r *http.Request) {
	if h.optionReports == nil {
		writeError(w, http.StatusServiceUnavailable, "not_configured", "Отчёты по лицензиям сервисов не настроены.")
		return
	}
	days := defaultExpiryDays
	if raw := r.URL.Query().Get("days"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > maxExpiryDays {
			writeError(w, http.StatusBadRequest, "bad_request", "Окно напоминаний — от 1 до 366 дней.")
			return
		}
		days = value
	}

	reports, err := h.optionReports.All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось прочитать лицензии сервисов.")
		return
	}
	records, err := h.registry.All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось прочитать реестр.")
		return
	}
	clients := clientsByOwner(records)

	fetched := make([]licenseReportItem, 0, len(reports))
	for _, report := range reports {
		fetched = append(fetched, licenseReportItem{Type: report.Type, State: report.State,
			FetchedAt: report.FetchedAt.Format(time.RFC3339)})
	}

	tariffs := service.LicenseTariffs(reports)
	codes := make([]string, 0, len(tariffs))
	for code := range tariffs {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	subscribers := make([]licenseSubscriberItem, 0, len(codes))
	for _, code := range codes {
		item := licenseSubscriberItem{Code: code, Tariffs: make([]licenseTariffItem, 0, len(tariffs[code]))}
		for _, t := range tariffs[code] {
			item.Tariffs = append(item.Tariffs, licenseTariff(t))
		}
		subscribers = append(subscribers, item)
	}

	now := time.Now().UTC()
	expiring := []licenseExpiringItem{}
	for _, e := range service.ExpiringLicenses(tariffs, now, days) {
		expiring = append(expiring, licenseExpiringItem{SubscriberCode: e.Tariff.SubscriberCode,
			Tariff: licenseTariff(e.Tariff), DaysLeft: e.DaysLeft,
			Clients: nonNilClients(clients[e.Tariff.SubscriberCode])})
	}
	low := []licenseLowItem{}
	for _, l := range service.LowLicenses(tariffs, now) {
		low = append(low, licenseLowItem{SubscriberCode: l.Tariff.SubscriberCode, TariffName: l.Tariff.Name,
			OrgName: l.Tariff.OrgName, End: formatTime(l.Tariff.End), Option: licenseOption(l.Option),
			Over: l.Over, Clients: nonNilClients(clients[l.Tariff.SubscriberCode])})
	}

	writeJSON(w, http.StatusOK, map[string]any{"days": days, "reports": fetched,
		"subscribers": subscribers, "expiring": expiring, "low": low})
}

func licenseTariff(t service.LicenseTariff) licenseTariffItem {
	item := licenseTariffItem{Name: t.Name, TypeNumber: t.TypeNumber, OrgName: t.OrgName,
		OrgINN: t.OrgINN, OrgKPP: t.OrgKPP, Start: formatTime(t.Start), End: formatTime(t.End),
		Services: []string{}, Options: make([]licenseOptionItem, 0, len(t.Options))}
	seen := map[string]bool{}
	for _, o := range t.Options {
		if !seen[o.Type] {
			seen[o.Type] = true
			item.Services = append(item.Services, o.Type)
		}
		item.Options = append(item.Options, licenseOption(o))
	}
	return item
}

func licenseOption(o service.LicenseOption) licenseOptionItem {
	item := licenseOptionItem{Type: o.Type, Name: o.Name, Quantitative: o.Quantitative,
		Max: o.MaxVolume, Used: o.UsedVolume}
	if remaining, ok := o.Remaining(); ok {
		item.Remaining = &remaining
	}
	return item
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

package httpapi

import (
	"context"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"time"

	"partnerops/internal/money"
	"partnerops/internal/service"
)

// dashboardEntry — пункт сводки и клиенты, к которым он относится: по ним
// сводка ставит ссылки на карточки. Пункт уровня абонента (договор, лицензия)
// относится ко всем его организациям.
type dashboardEntry[T any] struct {
	Clients []clientRef `json:"clients"`
	Item    T           `json:"item"`
}

func entryOf[T any](item T, clients ...*clientEntry) dashboardEntry[T] {
	refs := []clientRef{}
	for _, e := range clients {
		if e != nil {
			refs = append(refs, e.clientRef)
		}
	}
	return dashboardEntry[T]{Clients: refs, Item: item}
}

func entryFor[T any](item T, refs []clientRef) dashboardEntry[T] {
	return dashboardEntry[T]{Clients: refs, Item: item}
}

type dashboardCounts struct {
	Invoice      int `json:"invoice"`
	LowRemainder int `json:"lowRemainder"`
	Forecast     int `json:"forecast"`
	Advice       int `json:"advice"`
	Renewals     int `json:"renewals"`
	Industry     int `json:"industry"`
	Licenses     int `json:"licenses"`
	Anomalies    int `json:"anomalies"`
	ReviewDue    int `json:"reviewDue"`
	// EDOWithoutBilling и NotInBase — разрывы сверки базы абонентов с биллингом.
	EDOWithoutBilling int `json:"edoWithoutBilling"`
	NotInBase         int `json:"notInBase"`
	// Drafts и Exported — заявки, которые ещё не ушли: черновики и выгруженные файлом.
	Drafts   int `json:"drafts"`
	Exported int `json:"exported"`
}

// gapSubscriberItem — абонент с ЭДО (направление в 1С или трафик), которого
// нет в биллинге последнего месяца.
type gapSubscriberItem struct {
	Code      string   `json:"code"`
	Name      string   `json:"name"`
	InTraffic bool     `json:"inTraffic"`
	EDOIDs    []string `json:"edoIds"`
}

type dashboardGaps struct {
	// Period — последний месяц биллинга в реестре.
	Period            string                              `json:"period"`
	EDOWithoutBilling []dashboardEntry[gapSubscriberItem] `json:"edoWithoutBilling"`
	NotInBase         []dashboardEntry[notInBaseItem]     `json:"notInBase"`
}

// dashboardSources — когда обновлялся каждый источник сводки; пусто — ещё ни разу.
type dashboardSources struct {
	Billing     string `json:"billing,omitempty"`
	Subscribers string `json:"subscribers,omitempty"`
	ITS         string `json:"its,omitempty"`
	Traffic     string `json:"traffic,omitempty"`
	EPDUsage    string `json:"epdUsage,omitempty"`
	Licenses    string `json:"licenses,omitempty"`
}

type dashboardBilling struct {
	Period       string                                  `json:"period"`
	TakenAt      string                                  `json:"takenAt,omitempty"`
	TotalDue     string                                  `json:"totalDue"`
	OverLimit    []dashboardEntry[service.BillingClient] `json:"overLimit"`
	LowRemainder []dashboardEntry[service.BillingClient] `json:"lowRemainder"`
}

type dashboardForecast struct {
	Period string                             `json:"period,omitempty"`
	AsOf   string                             `json:"asOf,omitempty"`
	Items  []dashboardEntry[service.Forecast] `json:"items"`
}

type dashboardRenewals struct {
	Days      int                               `json:"days"`
	CheckedAt string                            `json:"checkedAt,omitempty"`
	Items     []dashboardEntry[itsExpiringItem] `json:"items"`
}

type dashboardLicenses struct {
	Expiring []dashboardEntry[licenseExpiringItem] `json:"expiring"`
	Low      []dashboardEntry[licenseLowItem]      `json:"low"`
}

type dashboardPayload struct {
	Counts    dashboardCounts                       `json:"counts"`
	Billing   dashboardBilling                      `json:"billing"`
	Forecast  dashboardForecast                     `json:"forecast"`
	Advice    []dashboardEntry[epdAdviceItem]       `json:"advice"`
	Renewals  dashboardRenewals                     `json:"renewals"`
	Industry  []dashboardEntry[industryMissingItem] `json:"industry"`
	Licenses  dashboardLicenses                     `json:"licenses"`
	Anomalies []dashboardEntry[anomalyItem]         `json:"anomalies"`
	Gaps      dashboardGaps                         `json:"gaps"`
	// Requests — неотправленные заявки, свежие сверху.
	Requests []dashboardEntry[requestListItem] `json:"requests"`
	Sources  dashboardSources                  `json:"sources"`
}

// Dashboard отдаёт сводку «что сделать» по всем клиентам: счета сверх лимита,
// малые остатки, прогноз, выгодный тариф, продления 1С:ИТС, ИТС Отраслевой,
// лицензии сервисов и активные находки. Только сохранённые данные, без 1С.
func (h *Registry) Dashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	days := defaultExpiryDays
	if raw := r.URL.Query().Get("days"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > maxExpiryDays {
			writeError(w, http.StatusBadRequest, "bad_request", "Окно напоминаний — от 1 до 366 дней.")
			return
		}
		days = value
	}
	d, err := h.loadClientData(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось прочитать данные клиентов.")
		return
	}
	now := time.Now().UTC()
	p := dashboardPayload{
		Billing: dashboardBilling{Period: d.billing.Period, TakenAt: d.billing.TakenAt,
			OverLimit: []dashboardEntry[service.BillingClient]{}, LowRemainder: []dashboardEntry[service.BillingClient]{}},
		Forecast:  dashboardForecast{Items: []dashboardEntry[service.Forecast]{}},
		Advice:    []dashboardEntry[epdAdviceItem]{},
		Renewals:  dashboardRenewals{Days: days, Items: []dashboardEntry[itsExpiringItem]{}},
		Industry:  []dashboardEntry[industryMissingItem]{},
		Licenses:  dashboardLicenses{Expiring: []dashboardEntry[licenseExpiringItem]{}, Low: []dashboardEntry[licenseLowItem]{}},
		Anomalies: []dashboardEntry[anomalyItem]{},
		Requests:  unsentRequests(d),
		Sources:   dashboardSources{Billing: d.billing.TakenAt, Subscribers: formatTime(d.fetched)},
	}
	if p.Gaps, err = h.billingGaps(ctx, d); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось сверить базу абонентов с биллингом.")
		return
	}
	if p.Sources.Traffic, err = runFetched(ctx, h.monthTraffic); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось прочитать отчёт трафика.")
		return
	}

	var due money.Amount
	for _, b := range d.billing.Clients {
		due += b.ClientAmount
		e := d.index.forRow(b.EDOID, b.INN, b.KPP)
		switch {
		case b.OverLimit:
			p.Billing.OverLimit = append(p.Billing.OverLimit, entryOf(b, e))
		case b.LowRemainder:
			p.Billing.LowRemainder = append(p.Billing.LowRemainder, entryOf(b, e))
		}
	}
	p.Billing.TotalDue = due.String()
	// Сначала те, кому выставить больше.
	sort.SliceStable(p.Billing.OverLimit, func(i, j int) bool {
		return p.Billing.OverLimit[i].Item.ClientAmount > p.Billing.OverLimit[j].Item.ClientAmount
	})

	forecast, ok, err := h.monthForecast(ctx, now)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось посчитать прогноз.")
		return
	}
	if ok {
		p.Forecast.Period, p.Forecast.AsOf = forecast.Period, forecast.AsOf.Format(time.RFC3339)
		for _, f := range forecast.Clients {
			if f.Warn {
				p.Forecast.Items = append(p.Forecast.Items, entryOf(f, d.index.forRow(f.EDOID, f.INN, f.KPP)))
			}
		}
	}

	if h.epdUsage != nil {
		if run, ok, err := h.epdUsage.Run(ctx); err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "Не удалось прочитать расход ЭПД.")
			return
		} else if ok {
			p.Sources.EPDUsage = formatTime(run.FetchedAt)
			rows, err := h.epdUsage.Rows(ctx)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal", "Не удалось прочитать расход ЭПД.")
				return
			}
			for _, a := range service.BuildEPDAdvice(rows, d.ids) {
				if a.AlreadyOptimal || a.SavingsKopeks <= 0 {
					continue
				}
				p.Advice = append(p.Advice, entryOf(epdAdviceFrom(a), d.index.forRequisites(a.INN, a.KPP)))
			}
		}
	}

	var checkedAt time.Time
	for _, check := range d.checks {
		if check.CheckedAt.After(checkedAt) {
			checkedAt = check.CheckedAt
		}
	}
	p.Renewals.CheckedAt = formatTime(checkedAt)
	for _, e := range service.ExpiringContracts(d.checks, now, days) {
		owned := itsClientsOf(d.index, e.SubscriberCode)
		p.Renewals.Items = append(p.Renewals.Items, entryFor(itsExpiringItem{
			SubscriberCode: e.SubscriberCode, Contract: contractItem(e.Contract), DaysLeft: e.DaysLeft,
			Clients: owned, Renewal: renewal(e.Contract, withTariffs(d, owned)),
		}, d.index.refsOf(e.SubscriberCode)))
	}
	for code, item := range d.industry {
		if len(item.Missing) > 0 {
			p.Industry = append(p.Industry, entryFor(industryMissingItem{SubscriberCode: code,
				Programs: item.Missing, Clients: itsClientsOf(d.index, code)}, d.index.refsOf(code)))
		}
	}
	sort.SliceStable(p.Industry, func(i, j int) bool {
		return p.Industry[i].Item.SubscriberCode < p.Industry[j].Item.SubscriberCode
	})

	if h.optionReports != nil {
		reports, err := h.optionReports.All(ctx)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "Не удалось прочитать лицензии сервисов.")
			return
		}
		var fetched time.Time
		for _, report := range reports {
			if report.FetchedAt.After(fetched) {
				fetched = report.FetchedAt
			}
		}
		p.Sources.Licenses = formatTime(fetched)
		tariffs := service.LicenseTariffs(reports)
		for _, e := range service.ExpiringLicenses(tariffs, now, days) {
			code := e.Tariff.SubscriberCode
			p.Licenses.Expiring = append(p.Licenses.Expiring, entryFor(licenseExpiringItem{SubscriberCode: code,
				Tariff: licenseTariff(e.Tariff), DaysLeft: e.DaysLeft, Clients: itsClientsOf(d.index, code)},
				licenseRefs(d.index, e.Tariff)))
		}
		for _, l := range service.LowLicenses(tariffs, now) {
			code := l.Tariff.SubscriberCode
			p.Licenses.Low = append(p.Licenses.Low, entryFor(licenseLowItem{SubscriberCode: code,
				TariffName: l.Tariff.Name, OrgName: l.Tariff.OrgName, End: formatTime(l.Tariff.End),
				Option: licenseOption(l.Option), Over: l.Over, Clients: itsClientsOf(d.index, code)},
				licenseRefs(d.index, l.Tariff)))
		}
	}

	for _, a := range d.anomalies {
		if a.ReviewDue {
			p.Counts.ReviewDue++
		}
		if a.hidden() {
			continue
		}
		p.Anomalies = append(p.Anomalies, entryOf(a, d.index.forRow(a.EDOID, a.INN, a.KPP)))
	}

	p.Counts.Invoice, p.Counts.LowRemainder = len(p.Billing.OverLimit), len(p.Billing.LowRemainder)
	p.Counts.Forecast, p.Counts.Advice = len(p.Forecast.Items), len(p.Advice)
	p.Counts.Renewals, p.Counts.Industry = len(p.Renewals.Items), len(p.Industry)
	p.Counts.Licenses = len(p.Licenses.Expiring) + len(p.Licenses.Low)
	p.Counts.Anomalies = len(p.Anomalies)
	p.Counts.EDOWithoutBilling, p.Counts.NotInBase = len(p.Gaps.EDOWithoutBilling), len(p.Gaps.NotInBase)
	p.Sources.ITS = p.Renewals.CheckedAt
	for _, r := range p.Requests {
		if r.Item.ExportedAt != "" {
			p.Counts.Exported++
		} else {
			p.Counts.Drafts++
		}
	}
	writeJSON(w, http.StatusOK, p)
}

// billingGaps сверяет базу абонентов с биллингом, как экран абонентов делал
// раньше: абоненты с ЭДО без биллинга и организации биллинга, которых нет в базе.
func (h *Registry) billingGaps(ctx context.Context, d *clientData) (dashboardGaps, error) {
	gaps := dashboardGaps{EDOWithoutBilling: []dashboardEntry[gapSubscriberItem]{},
		NotInBase: []dashboardEntry[notInBaseItem]{}}
	if len(d.subs) == 0 {
		return gaps, nil // базу ещё не выгружали: «нет в базе» были бы все
	}
	traffic, err := h.trafficRows(ctx)
	if err != nil {
		return gaps, err
	}
	check := service.CheckSubscribers(d.subs, d.ids, traffic)
	gaps.Period = check.Period
	for _, sub := range d.subs {
		cover := check.Coverage[sub.Code]
		if !cover.EDO || cover.InBilling || sub.LastSeenAt.Before(d.fetched) {
			continue
		}
		gaps.EDOWithoutBilling = append(gaps.EDOWithoutBilling, entryFor(gapSubscriberItem{Code: sub.Code,
			Name: sub.Name, InTraffic: cover.InTraffic, EDOIDs: nonNil(cover.EDOIDs)}, d.index.refsOf(sub.Code)))
	}
	for _, c := range check.NotInBase {
		var e *clientEntry
		if len(c.EDOIDs) > 0 {
			e = d.index.forRow(c.EDOIDs[0], c.INN, c.KPP)
		}
		gaps.NotInBase = append(gaps.NotInBase, entryOf(notInBaseItem{ClientName: c.ClientName, INN: c.INN, KPP: c.KPP,
			Logins: nonNil(c.Logins), OwnerCodes: nonNil(c.OwnerCodes), EDOIDs: nonNil(c.EDOIDs)}, e))
	}
	return gaps, nil
}

// unsentRequests — черновики и выгруженные, но не отправленные заявки с их клиентами.
func unsentRequests(d *clientData) []dashboardEntry[requestListItem] {
	list := []dashboardEntry[requestListItem]{}
	for _, r := range d.requests {
		if r.item.SentAt != "" {
			continue
		}
		refs := []clientRef{}
		for _, row := range r.rows {
			if e := d.index.forRequisites(row.INN, row.KPP); e != nil &&
				!slices.ContainsFunc(refs, func(ref clientRef) bool { return ref.Key == e.Key }) {
				refs = append(refs, e.clientRef)
			}
		}
		list = append(list, entryFor(r.item, refs))
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].Item.UpdatedAt > list[j].Item.UpdatedAt })
	return list
}

// runFetched — когда построен сохранённый отчёт; пусто — ещё не строился.
func runFetched(ctx context.Context, reader EPDUsageReader) (string, error) {
	if reader == nil {
		return "", nil
	}
	run, ok, err := reader.Run(ctx)
	if err != nil || !ok {
		return "", err
	}
	return formatTime(run.FetchedAt), nil
}

// licenseRefs — тариф на организацию относится к ней, без организации — ко всему абоненту.
func licenseRefs(x *clientIndex, t service.LicenseTariff) []clientRef {
	refs := []clientRef{}
	for _, e := range x.bySubscriber[t.SubscriberCode] {
		if licenseBelongs(t, e) {
			refs = append(refs, e.clientRef)
		}
	}
	return refs
}

// withTariffs дописывает организациям строку «Тарифы ИТС» из реестра: по ней
// продление подставляет тариф ЭПД.
func withTariffs(d *clientData, clients []itsClientItem) []itsClientItem {
	out := make([]itsClientItem, len(clients))
	for i, c := range clients {
		c.Tariffs = tariffsOf(d, clientKey(c.INN, c.KPP))
		out[i] = c
	}
	return out
}

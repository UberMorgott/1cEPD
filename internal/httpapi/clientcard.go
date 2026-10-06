package httpapi

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"partnerops/internal/itsreq"
	"partnerops/internal/money"
	"partnerops/internal/partner"
	"partnerops/internal/service"
	"partnerops/internal/store"
)

// RequestLister — сохранённые заявки: карточка клиента показывает его заявки.
type RequestLister interface {
	List(ctx context.Context) ([]store.RequestDraft, error)
}

// ProgramReader — программы клиентов по последней проверке в 1С.
type ProgramReader interface {
	All(ctx context.Context) ([]store.ClientProgram, error)
}

// WithClientCards добавляет в карточку клиента заявки и программы. Без них
// карточка работает, эти разделы пусты.
func (h *Registry) WithClientCards(requests RequestLister, programs ProgramReader) *Registry {
	h.requests, h.programs = requests, programs
	return h
}

// clientData — сохранённые данные всех экранов, из которых собираются реестр
// клиентов, карточка и сводка. Только чтение базы: 1С здесь не спрашивается,
// отчёты не строятся — их держат свежими фоновые задачи.
type clientData struct {
	index    *clientIndex
	ids      []store.IdentifierRecord
	subs     []store.SubscriberRecord
	fetched  time.Time
	requests []clientRequest
	programs []store.ClientProgram
	checks   []store.ITSCheck
	industry map[string]*industryItem
	billing  clientBilling
	// imported — загруженная выгрузка биллинга ЭПД; epdImport — когда и что.
	imported  []partner.EPDBillingRow
	epdImport *store.EPDImportInfo
	// anomalies — все находки, со скрытыми.
	anomalies []anomalyItem
}

type clientRequest struct {
	item requestListItem
	rows []itsreq.Row
}

type clientBilling struct {
	Period  string                  `json:"period"`
	TakenAt string                  `json:"takenAt,omitempty"`
	Clients []service.BillingClient `json:"clients"`
}

// loadClientData читает всё сохранённое. Не подключённые источники пусты.
func (h *Registry) loadClientData(ctx context.Context) (*clientData, error) {
	d := &clientData{industry: map[string]*industryItem{}}
	var err error
	if d.ids, err = h.registry.All(ctx); err != nil {
		return nil, err
	}
	if h.subscribers != nil {
		if d.subs, err = h.subscribers.All(ctx); err != nil {
			return nil, err
		}
		if d.fetched, err = h.subscribers.LastFetched(ctx); err != nil {
			return nil, err
		}
	}
	var requestRows []itsreq.Row
	if h.requests != nil {
		drafts, err := h.requests.List(ctx)
		if err != nil {
			return nil, err
		}
		for _, draft := range drafts {
			item, rows := requestItemFrom(draft)
			d.requests = append(d.requests, clientRequest{item: item, rows: rows})
			requestRows = append(requestRows, rows...)
		}
	}
	if h.programs != nil {
		if d.programs, err = h.programs.All(ctx); err != nil {
			return nil, err
		}
	}
	if h.itsChecks != nil {
		if d.checks, err = h.itsChecks.All(ctx); err != nil {
			return nil, err
		}
	}
	if h.industryChecks != nil {
		found, err := h.industryChecks.All(ctx)
		if err != nil {
			return nil, err
		}
		for _, check := range found {
			d.industry[check.SubscriberCode] = industryFrom(check)
		}
	}
	d.billing = clientBilling{Period: previousPeriod(time.Now()), Clients: []service.BillingClient{}}
	if h.snapshots != nil {
		snapshot, rows, err := h.snapshots.Latest(ctx, d.billing.Period)
		switch {
		case errors.Is(err, store.ErrNoSnapshot):
		case err != nil:
			return nil, err
		default:
			d.billing.TakenAt = formatTime(snapshot.TakenAt)
			d.billing.Clients = service.BuildBilling(rows).Clients
		}
	}
	if d.anomalies, err = h.anomalyItems(ctx); err != nil {
		return nil, err
	}
	var traffic []partner.EDOTrafficRow
	for _, reader := range []EPDUsageReader{h.monthTraffic, h.epdUsage} {
		if reader == nil {
			continue
		}
		rows, err := reader.Rows(ctx)
		if err != nil {
			return nil, err
		}
		traffic = append(traffic, rows...)
	}
	if h.epdImport != nil {
		if d.imported, err = h.epdImport.Rows(ctx); err != nil {
			return nil, err
		}
		info, ok, err := h.epdImport.Last(ctx)
		if err != nil {
			return nil, err
		}
		if ok {
			d.epdImport = &info
		}
	}
	today := time.Now().In(moscow).Format(time.DateOnly)
	d.index = buildClientIndex(d.ids, traffic, d.imported, d.subs, requestRows, today)
	return d, nil
}

// requestsOf — заявки, в строках которых есть клиент.
func (d *clientData) requestsOf(e *clientEntry) []requestListItem {
	list := []requestListItem{}
	for _, r := range d.requests {
		for _, row := range r.rows {
			if d.index.forRequisites(row.INN, row.KPP) == e {
				list = append(list, r.item)
				break
			}
		}
	}
	return list
}

// clientListItem — строка реестра клиентов.
type clientListItem struct {
	clientRef
	SubscriberCodes []string `json:"subscriberCodes"`
	// SubscriberName — название первого абонента из базы; пусто — абонента в базе нет.
	SubscriberName string   `json:"subscriberName"`
	Logins         []string `json:"logins"`
	EDOIDs         []string `json:"edoIds"`
	Sources        []string `json:"sources"`
	InBase         bool     `json:"inBase"`
	// Ours — наш клиент 1С-ЭДО: у него есть наш идентификатор ЭДО, см. ourEDOIDs.
	Ours bool `json:"ours"`
	// InBilling — есть строки в биллинге последнего закрытого месяца; ниже их итоги.
	InBilling    bool   `json:"inBilling"`
	Limit        *int64 `json:"limit"`
	Used         int64  `json:"used"`
	Billable     int64  `json:"billable"`
	Amount       string `json:"amount"`
	OverLimit    bool   `json:"overLimit"`
	LowRemainder bool   `json:"lowRemainder"`
	// ITSEnd — самый поздний конец договора 1С:ИТС у абонентов клиента.
	ITSEnd    string `json:"itsEnd,omitempty"`
	Anomalies int    `json:"anomalies"`
	Requests  int    `json:"requests"`
}

// ClientList отдаёт реестр клиентов: одна строка — одна организация.
func (h *Registry) ClientList(w http.ResponseWriter, r *http.Request) {
	d, err := h.loadClientData(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось прочитать данные клиентов.")
		return
	}
	names := map[string]string{}
	for _, sub := range d.subs {
		names[sub.Code] = sub.Name
	}
	itsEnd := map[string]time.Time{}
	for _, check := range d.checks {
		for _, c := range check.Contracts {
			if c.End.After(itsEnd[check.SubscriberCode]) {
				itsEnd[check.SubscriberCode] = c.End
			}
		}
	}

	items := make([]clientListItem, 0, len(d.index.list))
	at := map[*clientEntry]int{}
	amounts := make([]money.Amount, len(d.index.list))
	for i, e := range d.index.list {
		at[e] = i
		item := clientListItem{clientRef: e.clientRef, SubscriberCodes: nonNil(e.SubscriberCodes),
			Logins: nonNil(e.Logins), EDOIDs: nonNil(e.EDOIDs), Sources: nonNil(e.Sources), InBase: e.InBase, Ours: e.Ours}
		var end time.Time
		for _, code := range e.SubscriberCodes {
			item.SubscriberName = firstNonEmpty(item.SubscriberName, names[code])
			if itsEnd[code].After(end) {
				end = itsEnd[code]
			}
		}
		item.ITSEnd = formatTime(end)
		items = append(items, item)
	}
	for _, b := range d.billing.Clients {
		e := d.index.forRow(b.EDOID, b.INN, b.KPP)
		if e == nil {
			continue
		}
		item := &items[at[e]]
		item.InBilling = true
		if b.Limit != nil {
			limit := *b.Limit
			if item.Limit != nil {
				limit += *item.Limit
			}
			item.Limit = &limit
		}
		item.Used += b.Used
		item.Billable += b.Billable
		amounts[at[e]] += b.ClientAmount
		item.OverLimit = item.OverLimit || b.OverLimit
		item.LowRemainder = item.LowRemainder || b.LowRemainder
	}
	for _, a := range d.anomalies {
		if e := d.index.forRow(a.EDOID, a.INN, a.KPP); e != nil && !a.hidden() {
			items[at[e]].Anomalies++
		}
	}
	for _, r := range d.requests {
		seen := map[*clientEntry]bool{}
		for _, row := range r.rows {
			if e := d.index.forRequisites(row.INN, row.KPP); e != nil && !seen[e] {
				seen[e] = true
				items[at[e]].Requests++
			}
		}
	}
	for i := range items {
		items[i].Amount = amounts[i].String()
	}

	response := map[string]any{"period": d.billing.Period, "clients": items}
	if d.billing.TakenAt != "" {
		response["takenAt"] = d.billing.TakenAt
	}
	if !d.fetched.IsZero() {
		response["subscribersFetchedAt"] = d.fetched.Format(time.RFC3339)
	}
	if d.epdImport != nil {
		response["epdImport"] = epdImportJSON(*d.epdImport)
	}
	writeJSON(w, http.StatusOK, response)
}

// clientSubscriberItem — абонент клиента. InBase false — код известен только
// из биллинга и отчётов трафика: в базе абонентов 1С его нет, имя — оттуда же.
type clientSubscriberItem struct {
	Code       string   `json:"code"`
	Name       string   `json:"name"`
	Subjects   []string `json:"subjects"`
	RegNumbers []string `json:"regNumbers"`
	InBase     bool     `json:"inBase"`
	Gone       bool     `json:"gone"`
	// Organizations — все организации абонента, включая эту.
	Organizations []clientRef `json:"organizations"`
}

// clientCardPayload — всё о клиенте из сохранённых данных.
type clientCardPayload struct {
	clientRef
	Sources []string `json:"sources"`
	InBase  bool     `json:"inBase"`
	// Ours — наш клиент 1С-ЭДО: его абонент-владелец известен по нашему ЭДО,
	// даже если базы абонентов 1С в нём нет.
	Ours        bool                   `json:"ours"`
	Logins      []string               `json:"logins"`
	Subscribers []clientSubscriberItem `json:"subscribers"`
	// SameINN — другие КПП того же ИНН: отдельные карточки.
	SameINN     []clientRef      `json:"sameInn"`
	Identifiers []identifierItem `json:"identifiers"`
	Billing     clientBilling    `json:"billing"`
	// History — 12 закрытых месяцев: итоги всех клиентов и расход идентификаторов этого.
	History  service.BillingHistory `json:"history"`
	Forecast *service.MonthForecast `json:"forecast"`
	Advice   *epdAdviceItem         `json:"advice"`
	// ITS — договоры 1С:ИТС и ИТС Отраслевой абонентов клиента; ITSExpiring —
	// кончаются в ближайшие 30 суток; Renewal — предзаполнение продления по
	// самому позднему договору.
	ITS         []itsSubscriberItem `json:"its"`
	ITSExpiring []itsExpiringItem   `json:"itsExpiring"`
	Renewal     *renewalDraft       `json:"renewal"`
	Licenses    []licenseTariffItem `json:"licenses"`
	LicensesLow []licenseLowItem    `json:"licensesLow"`
	Programs    []clientProgram     `json:"programs"`
	// ProgramsLogin и ProgramsCheckedAt — чем и когда программы проверялись в 1С.
	ProgramsLogin     string            `json:"programsLogin,omitempty"`
	ProgramsCheckedAt string            `json:"programsCheckedAt,omitempty"`
	Anomalies         []anomalyItem     `json:"anomalies"`
	Requests          []requestListItem `json:"requests"`
}

// ClientCard отдаёт карточку клиента по ключу (см. clientKey).
func (h *Registry) ClientCard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	d, err := h.loadClientData(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось прочитать данные клиентов.")
		return
	}
	e := d.index.resolve(r.PathValue("key"))
	if e == nil {
		writeError(w, http.StatusNotFound, "not_found", "Клиент не найден.")
		return
	}
	card, err := h.buildCard(ctx, d, e)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось собрать карточку клиента.")
		return
	}
	writeJSON(w, http.StatusOK, card)
}

func (h *Registry) buildCard(ctx context.Context, d *clientData, e *clientEntry) (clientCardPayload, error) {
	card := clientCardPayload{
		clientRef: e.clientRef, Sources: nonNil(e.Sources), InBase: e.InBase, Ours: e.Ours, Logins: nonNil(e.Logins),
		Subscribers: []clientSubscriberItem{}, SameINN: []clientRef{}, Identifiers: []identifierItem{},
		Billing: clientBilling{Period: d.billing.Period, TakenAt: d.billing.TakenAt, Clients: []service.BillingClient{}},
		ITS:     []itsSubscriberItem{}, ITSExpiring: []itsExpiringItem{},
		Licenses: []licenseTariffItem{}, LicensesLow: []licenseLowItem{}, Programs: []clientProgram{},
		Anomalies: []anomalyItem{}, Requests: d.requestsOf(e),
	}
	ids := e.EDOIDs
	codes := e.SubscriberCodes

	for _, sub := range d.subs {
		if !slices.Contains(codes, sub.Code) {
			continue
		}
		card.Subscribers = append(card.Subscribers, clientSubscriberItem{Code: sub.Code, Name: sub.Name,
			Subjects: nonNil(sub.Subjects), RegNumbers: nonNil(sub.RegNumbers), InBase: true,
			Gone: sub.LastSeenAt.Before(d.fetched), Organizations: d.index.refsOf(sub.Code)})
	}
	for _, code := range codes {
		if !slices.ContainsFunc(card.Subscribers, func(s clientSubscriberItem) bool { return s.Code == code }) {
			card.Subscribers = append(card.Subscribers, clientSubscriberItem{Code: code, Name: d.index.ownerNames[code],
				Subjects: []string{}, RegNumbers: []string{}, Organizations: d.index.refsOf(code)})
		}
	}
	if validINN(e.INN) {
		for _, other := range d.index.byINN[e.INN] {
			if other != e {
				card.SameINN = append(card.SameINN, other.clientRef)
			}
		}
	}

	traffic, err := h.trafficByID(ctx)
	if err != nil {
		return card, err
	}
	for _, id := range d.ids {
		if slices.Contains(ids, id.EDOID) {
			card.Identifiers = append(card.Identifiers, identifierFrom(id, traffic))
		}
	}
	for _, b := range d.billing.Clients {
		if d.index.forRow(b.EDOID, b.INN, b.KPP) == e {
			card.Billing.Clients = append(card.Billing.Clients, b)
		}
	}

	history, err := h.billingHistory(ctx)
	if err != nil {
		return card, err
	}
	own := []service.HistoryClient{}
	for _, c := range history.Clients {
		if slices.Contains(ids, c.EDOID) {
			own = append(own, c)
		}
	}
	history.Clients = own
	card.History = history

	forecast, ok, err := h.monthForecast(ctx, time.Now())
	if err != nil {
		return card, err
	}
	if ok {
		mine := []service.Forecast{}
		for _, f := range forecast.Clients {
			if slices.Contains(ids, f.EDOID) {
				mine = append(mine, f)
			}
		}
		forecast.Clients = mine
		card.Forecast = &forecast
	}

	if card.Advice, err = h.adviceFor(ctx, d, e); err != nil {
		return card, err
	}

	h.fillITS(&card, d, codes)
	if err := h.fillLicenses(ctx, &card, d, e); err != nil {
		return card, err
	}

	for _, p := range d.programs {
		if d.index.forRequisites(p.INN, p.KPP) != e {
			continue
		}
		card.Programs = append(card.Programs, programJSON(p))
		card.ProgramsLogin = p.Login
		card.ProgramsCheckedAt = p.CheckedAt.Format(time.RFC3339)
	}
	for _, a := range d.anomalies {
		if d.index.forRow(a.EDOID, a.INN, a.KPP) == e {
			card.Anomalies = append(card.Anomalies, a)
		}
	}
	return card, nil
}

// adviceFor — совет тарифа ЭПД организации; nil — расхода и тарифа ЭПД нет.
// Совет считается по ИНН+КПП, поэтому у записей без настоящего ИНН его нет.
func (h *Registry) adviceFor(ctx context.Context, d *clientData, e *clientEntry) (*epdAdviceItem, error) {
	if h.epdUsage == nil || !validINN(e.INN) {
		return nil, nil
	}
	_, ok, err := h.epdUsage.Run(ctx)
	if err != nil || !ok {
		return nil, err
	}
	rows, err := h.epdUsage.Rows(ctx)
	if err != nil {
		return nil, err
	}
	for _, a := range service.BuildEPDAdvice(rows, d.ids) {
		if d.index.forRequisites(a.INN, a.KPP) == e {
			item := epdAdviceFrom(a)
			return &item, nil
		}
	}
	return nil, nil
}

// fillITS кладёт в карточку договоры абонентов клиента и продление.
func (h *Registry) fillITS(card *clientCardPayload, d *clientData, codes []string) {
	var latest *partner.ITSContract
	var mine []store.ITSCheck
	for _, check := range d.checks {
		if !slices.Contains(codes, check.SubscriberCode) {
			continue
		}
		mine = append(mine, check)
		item := itsSubscriberItem{
			Code: check.SubscriberCode, StatusCode: check.Code, Status: check.Status,
			Description: check.Description, Contracts: make([]itsContractItem, 0, len(check.Contracts)),
			Clients: itsClientsOf(d.index, check.SubscriberCode), CheckedAt: check.CheckedAt.Format(time.RFC3339),
			Industry: d.industry[check.SubscriberCode],
		}
		for i, contract := range check.Contracts {
			item.Contracts = append(item.Contracts, contractItem(contract))
			if latest == nil || contract.End.After(latest.End) {
				latest = &check.Contracts[i]
			}
		}
		card.ITS = append(card.ITS, item)
	}
	own := []itsClientItem{{INN: card.INN, KPP: card.KPP, ClientName: card.ClientName, Tariffs: tariffsOf(d, card.Key)}}
	if latest != nil && !latest.End.IsZero() {
		draft := renewal(*latest, own)
		card.Renewal = &draft
	}
	for _, e := range service.ExpiringContracts(mine, time.Now().UTC(), defaultExpiryDays) {
		card.ITSExpiring = append(card.ITSExpiring, itsExpiringItem{
			SubscriberCode: e.SubscriberCode, Contract: contractItem(e.Contract), DaysLeft: e.DaysLeft,
			Clients: itsClientsOf(d.index, e.SubscriberCode), Renewal: renewal(e.Contract, own),
		})
	}
}

// fillLicenses — тарифы сервисов абонентов клиента. Тариф, выписанный на другую
// организацию того же абонента, сюда не попадает.
func (h *Registry) fillLicenses(ctx context.Context, card *clientCardPayload, d *clientData, e *clientEntry) error {
	if h.optionReports == nil {
		return nil
	}
	reports, err := h.optionReports.All(ctx)
	if err != nil {
		return err
	}
	all := service.LicenseTariffs(reports)
	tariffs := map[string][]service.LicenseTariff{}
	for _, code := range e.SubscriberCodes {
		for _, t := range all[code] {
			if licenseBelongs(t, e) {
				tariffs[code] = append(tariffs[code], t)
			}
		}
	}
	for _, code := range e.SubscriberCodes {
		for _, t := range tariffs[code] {
			card.Licenses = append(card.Licenses, licenseTariff(t))
		}
	}
	for _, l := range service.LowLicenses(tariffs, time.Now().UTC()) {
		card.LicensesLow = append(card.LicensesLow, licenseLowItem{SubscriberCode: l.Tariff.SubscriberCode,
			TariffName: l.Tariff.Name, OrgName: l.Tariff.OrgName, End: formatTime(l.Tariff.End),
			Option: licenseOption(l.Option), Over: l.Over, Clients: itsClientsOf(d.index, l.Tariff.SubscriberCode)})
	}
	return nil
}

// licenseBelongs — тариф без организации относится ко всему абоненту, с
// организацией — только к ней.
func licenseBelongs(t service.LicenseTariff, e *clientEntry) bool {
	inn := strings.TrimSpace(t.OrgINN)
	if inn == "" {
		return true
	}
	if inn != e.INN {
		return false
	}
	kpp := strings.TrimSpace(t.OrgKPP)
	return kpp == "" || kpp == e.KPP
}

// itsClientsOf — организации абонента в формате ответа о договорах.
func itsClientsOf(x *clientIndex, code string) []itsClientItem {
	list := []itsClientItem{}
	for _, e := range x.bySubscriber[code] {
		list = append(list, itsClientItem{INN: e.INN, KPP: e.KPP, ClientName: e.ClientName})
	}
	return list
}

// tariffsOf — строка «Тарифы ИТС» клиента из реестра: по ней продление
// подставляет тариф ЭПД.
func tariffsOf(d *clientData, key string) string {
	for _, id := range d.ids {
		if d.index.byEDO[id.EDOID] != nil && d.index.byEDO[id.EDOID].Key == key && id.ITSTariffs != "" {
			return id.ITSTariffs
		}
	}
	for _, row := range d.imported {
		if e := d.index.byEDO[row.EDOID]; e != nil && e.Key == key && row.ITSTariffs != "" {
			return row.ITSTariffs
		}
	}
	return ""
}

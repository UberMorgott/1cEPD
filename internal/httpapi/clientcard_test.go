package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"partnerops/internal/itsreq"
	"partnerops/internal/money"
	"partnerops/internal/partner"
	"partnerops/internal/store"
)

// Вымышленные реквизиты: форма та же, что в живых данных, цифры не чьи-то.
const (
	processorINN = "7700000011"
	processorKPP = "770001001"
	loneINN      = "7700000022" // в биллинге, но нет в базе абонентов
	idleINN      = "7700000033" // в базе абонентов, но нет в биллинге
	branchKPP    = "770002002"  // второе подразделение idleINN
)

type fixtureRegistry struct {
	ids    []store.IdentifierRecord
	events []store.AnomalyEvent
}

func (f *fixtureRegistry) Events(ctx context.Context, includeAcknowledged bool) ([]store.AnomalyEvent, error) {
	return f.events, nil
}

func (f *fixtureRegistry) All(ctx context.Context) ([]store.IdentifierRecord, error) {
	return f.ids, nil
}

func (f *fixtureRegistry) Acknowledge(context.Context, string, string, string, string, time.Time) error {
	return nil
}

type stubSubscribers []store.SubscriberRecord

func (s stubSubscribers) All(ctx context.Context) ([]store.SubscriberRecord, error) { return s, nil }
func (s stubSubscribers) LastFetched(ctx context.Context) (time.Time, error) {
	return time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), nil
}

type stubRequestList []store.RequestDraft

func (s stubRequestList) List(ctx context.Context) ([]store.RequestDraft, error) { return s, nil }

type stubPrograms []store.ClientProgram

func (s stubPrograms) All(ctx context.Context) ([]store.ClientProgram, error) { return s, nil }

func limitOf(v int64) *int64 { return new(v) }

func draftFor(t *testing.T, id int64, rows ...itsreq.Row) store.RequestDraft {
	t.Helper()
	payload, err := json.Marshal(itsreq.Request{PartnerCode: "P-1", Rows: rows}) //nolint:gosec // G117: в тестовой заявке пароля нет
	if err != nil {
		t.Fatal(err)
	}
	return store.RequestDraft{ID: id, Title: "Заявка", Status: "draft", Revision: 1,
		PayloadJSON: string(payload), UpdatedAt: time.Now()}
}

// fixtureRegistryAPI — живая раскладка в миниатюре: у «Центра переработки» два
// идентификатора ЭДО одной организации (у второго владелец не указан); клиент
// без абонента; абонент с двумя организациями без биллинга; две разные
// организации с нулевым ИНН — одна в реестре ЭДО, одна в базе абонентов.
func fixtureRegistryAPI(t *testing.T) *Registry {
	t.Helper()
	period := previousPeriod(time.Now())
	now := time.Now().UTC()
	ids := []store.IdentifierRecord{
		{EDOID: "2AE-R1", INN: processorINN, KPP: processorKPP, ClientName: `ООО "Центр переработки"`,
			Login: "processor@example.test", OwnerCode: "CL-100", Limit: limitOf(600), Packets: 650, LastPeriod: period,
			ITSTariffs: "1С-ЭДО. ЭПД-600(0): подп. ИТС №: 1"},
		{EDOID: "2AE-R2", INN: processorINN, KPP: processorKPP, ClientName: `ООО "Центр переработки"`,
			Login: "processor@example.test", LastPeriod: period},
		{EDOID: "2AE-L1", INN: loneINN, ClientName: "ИП Одинокий", OwnerCode: "CL-900", LastPeriod: period},
		{EDOID: "2AE-Z1", INN: "0000000000", KPP: "000000000", ClientName: "zero-a@example.test", LastPeriod: period},
		{EDOID: "2AE-Z2", INN: "0000000000", KPP: "000000000", ClientName: "zero-b@example.test", LastPeriod: period},
	}
	events := []store.AnomalyEvent{
		{ID: 1, EDOID: "2AE-R2", Kind: "orphan_with_traffic", INN: processorINN, KPP: processorKPP,
			StateFingerprint: "f1", DetectedAt: now},
		{ID: 2, EDOID: "2AE-Z1", Kind: "orphan_idle", INN: "0000000000", StateFingerprint: "f2", DetectedAt: now},
	}
	subs := stubSubscribers{
		{Code: "CL-100", Name: "Переработка-абонент", RegNumbers: []string{"800000001"},
			Organizations: []partner.Organization{
				{Name: `ООО "Центр переработки"`, INN: processorINN, KPP: processorKPP},
				// 1С повторяет организацию внутри абонента: карточка одна.
				{Name: `ООО "Центр переработки"`, INN: processorINN, KPP: processorKPP},
			}, LastSeenAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)},
		{Code: "CL-200", Name: "Тихий абонент", Subjects: []string{"EDO"}, Organizations: []partner.Organization{
			{Name: "ООО Тихое", INN: idleINN, KPP: processorKPP},
			{Name: "ООО Тихое, филиал", INN: idleINN, KPP: branchKPP},
			{Name: "zero-c@example.test", INN: "0000000000", KPP: "000000000"},
		}, LastSeenAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)},
	}
	snapshots := monthSnapshots{period: {
		{EDOID: "2AE-R1", INN: processorINN, KPP: processorKPP, ClientName: `ООО "Центр переработки"`,
			Limit: limitOf(600), Packets: 650, PacketsBillable: 50, ClientAmount: money.Amount(1_500_000)},
		{EDOID: "2AE-R2", INN: processorINN, KPP: processorKPP, ClientName: `ООО "Центр переработки"`, Packets: 3},
		{EDOID: "2AE-L1", INN: loneINN, ClientName: "ИП Одинокий", Limit: limitOf(100), Packets: 95},
	}}
	its := stubITSChecks{{SubscriberCode: "CL-100", Code: 1, Contracts: []partner.ITSContract{
		{TypeName: "ПРОФ", End: now.AddDate(0, 0, 10)},
	}, CheckedAt: now}}
	requests := stubRequestList{
		// КПП не введён, но у ИНН один клиент — заявка его.
		draftFor(t, 7, itsreq.Row{INN: processorINN, CompanyName: "Центр переработки"}),
		draftFor(t, 8, itsreq.Row{INN: "0000000000", CompanyName: "без ИНН"}),
		draftFor(t, 9, itsreq.Row{INN: "7700000044", KPP: "770001001", CompanyName: "Новый клиент"}),
	}
	programs := stubPrograms{{INN: processorINN, KPP: processorKPP, Login: "processor@example.test",
		RegNumber: "800000001", Program: "Бухгалтерия", HasAccess: true, CheckedAt: now}}
	usage := stubUsage{ok: true, rows: []partner.EDOTrafficRow{
		{EDOID: "2AE-R1", INN: processorINN, KPP: processorKPP, Subscriber: "CL-100: Переработка", EPDOut: 5000, Operator: "Калуга"},
	}}
	return NewRegistry(&fixtureRegistry{ids: ids, events: events}, snapshots).
		WithSubscribers(subs, nil).WithITS(its, nil).WithEPDUsage(usage).WithHistory(stubAttempts{}).
		WithClientCards(requests, programs)
}

func getJSON(t *testing.T, handler http.HandlerFunc, pattern, url string, out any) int {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc(pattern, handler)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, url, nil))
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			t.Fatalf("ответ %s: %v", rec.Body.String(), err)
		}
	}
	return rec.Code
}

func TestClientKey(t *testing.T) {
	cases := map[[2]string]string{
		{processorINN, processorKPP}: processorINN + "-" + processorKPP,
		{" 780700000416 ", ""}:       "780700000416", // ИП без КПП
		{processorINN, "000000000"}:  processorINN,
		{"0000000000", "000000000"}:  "",
		{"", processorKPP}:           "",
		{"77000000", processorKPP}:   "",
		{"77000000AB", processorKPP}: "",
		{"000000000000", ""}:         "",
	}
	for in, want := range cases {
		if got := clientKey(in[0], in[1]); got != want {
			t.Errorf("clientKey(%q, %q) = %q, ожидали %q", in[0], in[1], got, want)
		}
	}
}

func TestClientListOneRowPerOrganization(t *testing.T) {
	h := fixtureRegistryAPI(t)
	var body struct {
		Clients []clientListItem `json:"clients"`
	}
	if code := getJSON(t, h.ClientList, "GET /api/clients", "/api/clients", &body); code != http.StatusOK {
		t.Fatalf("код %d", code)
	}
	byKey := map[string]clientListItem{}
	for _, c := range body.Clients {
		if _, dup := byKey[c.Key]; dup {
			t.Fatalf("ключ %s повторяется", c.Key)
		}
		byKey[c.Key] = c
	}
	// Центр переработки, одиночка, два КПП тихого, две нулевых из реестра,
	// нулевая из базы, новый клиент из заявки. Заявка без ИНН клиентом не стала.
	if len(body.Clients) != 8 {
		t.Fatalf("клиентов %d: %+v", len(body.Clients), body.Clients)
	}

	rec := byKey[processorINN+"-"+processorKPP]
	if len(rec.EDOIDs) != 2 || !rec.InBase || !rec.InBilling || rec.Used != 653 || rec.Limit == nil || *rec.Limit != 600 ||
		!rec.OverLimit || rec.Amount != money.Amount(1_500_000).String() || rec.Anomalies != 1 || rec.Requests != 1 ||
		rec.SubscriberName != "Переработка-абонент" || rec.ITSEnd == "" {
		t.Errorf("центр переработки: %+v", rec)
	}
	if lone := byKey[loneINN]; lone.InBase || !lone.InBilling || !lone.LowRemainder || lone.SubscriberCodes[0] != "CL-900" {
		t.Errorf("клиент без абонента: %+v", lone)
	}
	if idle := byKey[idleINN+"-"+processorKPP]; !idle.InBase || idle.InBilling || idle.Amount != "0,00" {
		t.Errorf("клиент без биллинга: %+v", idle)
	}
	for _, key := range []string{"edo-2AE-Z1", "edo-2AE-Z2"} {
		if z, ok := byKey[key]; !ok || len(z.EDOIDs) != 1 {
			t.Errorf("нулевой ИНН %s склеен или потерян: %+v", key, z)
		}
	}
	if z := byKey["edo-2AE-Z1"]; z.Anomalies != 1 {
		t.Errorf("находка нулевого ИНН ушла не туда: %+v", z)
	}
	if n := byKey["7700000044-770001001"]; len(n.Sources) != 1 || n.Sources[0] != sourceRequest {
		t.Errorf("клиент только из заявки: %+v", n)
	}
}

// Наши клиенты определяются сами, по данным партнёрского API: идентификаторы
// биллинга и отчётов трафика. База абонентов и заявки нашими не делают.
func TestClientListMarksOursAutomatically(t *testing.T) {
	month := stubUsage{ok: true, rows: []partner.EDOTrafficRow{
		// Связь создана после закрытого биллинга: в реестре её нет.
		{EDOID: "2AE-NEW", INN: "7700000066", KPP: "770001001", ClientName: "ООО Новая связь",
			Subscriber: "CL-300: Новый абонент", Login: "new@example.test", SupportFrom: "2026-09-07"},
		// Сопровождение одиночки кончилось.
		{EDOID: "2AE-L1", INN: loneINN, SupportTo: "2020-01-01"},
	}}
	h := fixtureRegistryAPI(t).WithForecast(month)
	var body struct {
		Clients []clientListItem `json:"clients"`
	}
	if code := getJSON(t, h.ClientList, "GET /api/clients", "/api/clients", &body); code != http.StatusOK {
		t.Fatalf("код %d", code)
	}
	ours := map[string]bool{}
	byKey := map[string]clientListItem{}
	for _, c := range body.Clients {
		byKey[c.Key] = c
		if c.Ours {
			ours[c.Key] = true
		}
	}
	want := []string{processorINN + "-" + processorKPP, "edo-2AE-Z1", "edo-2AE-Z2", "7700000066-770001001"}
	for _, key := range want {
		if !ours[key] {
			t.Errorf("%s не наш: %v", key, ours)
		}
	}
	if len(ours) != len(want) {
		t.Errorf("наших %d, ожидали %d: %v", len(ours), len(want), ours)
	}
	fresh := byKey["7700000066-770001001"]
	if fresh.ClientName != "ООО Новая связь" || fresh.SubscriberCodes[0] != "CL-300" || fresh.Sources[0] != sourceTraffic {
		t.Errorf("клиент из отчёта трафика: %+v", fresh)
	}
}

func TestOurEDOIDsDropsVanishedAndEnded(t *testing.T) {
	ids := []store.IdentifierRecord{
		{EDOID: "A", LastPeriod: "2026-08"},
		{EDOID: "GONE", LastPeriod: "2026-06"},
		{EDOID: "ENDED", LastPeriod: "2026-08"},
		{EDOID: "RENEWED", LastPeriod: "2026-08"},
	}
	traffic := []partner.EDOTrafficRow{
		{EDOID: "ENDED", SupportTo: "2026-09-01"},
		{EDOID: "RENEWED", SupportTo: "2026-09-01"},
		{EDOID: "RENEWED", SupportTo: "2027-09-01"},
		{EDOID: "LATE"},
	}
	got := ourEDOIDs(ids, traffic, "2026-09-30")
	want := map[string]bool{"A": true, "RENEWED": true, "LATE": true}
	if len(got) != len(want) {
		t.Fatalf("наши %v, ожидали %v", got, want)
	}
	for id := range want {
		if !got[id] {
			t.Errorf("%s не наш: %v", id, got)
		}
	}
}
func TestClientCardAggregatesEverything(t *testing.T) {
	h := fixtureRegistryAPI(t)
	var card clientCardPayload
	url := "/api/clients/" + processorINN + "-" + processorKPP
	if code := getJSON(t, h.ClientCard, "GET /api/clients/{key}", url, &card); code != http.StatusOK {
		t.Fatalf("код %d", code)
	}
	if len(card.Identifiers) != 2 || card.Identifiers[0].Traffic == nil || card.Identifiers[0].Traffic.Operator != "Калуга" {
		t.Errorf("идентификаторы: %+v", card.Identifiers)
	}
	if len(card.Billing.Clients) != 2 || len(card.History.Months) != 12 || len(card.History.Clients) != 2 {
		t.Errorf("биллинг %+v, история %+v", card.Billing, card.History.Clients)
	}
	if len(card.Subscribers) != 1 || card.Subscribers[0].Code != "CL-100" || !card.Subscribers[0].InBase ||
		len(card.Subscribers[0].Organizations) != 1 || card.Subscribers[0].RegNumbers[0] != "800000001" {
		t.Errorf("абонент: %+v", card.Subscribers)
	}
	if card.Advice == nil || card.Advice.EPDOut != 5000 {
		t.Errorf("совет тарифа: %+v", card.Advice)
	}
	if len(card.ITS) != 1 || len(card.ITSExpiring) != 1 || card.Renewal == nil || card.Renewal.TariffCode == "" {
		t.Errorf("ИТС %+v, продление %+v / %+v", card.ITS, card.ITSExpiring, card.Renewal)
	}
	if len(card.Programs) != 1 || card.ProgramsLogin != "processor@example.test" {
		t.Errorf("программы: %+v", card.Programs)
	}
	if len(card.Anomalies) != 1 || card.Anomalies[0].EDOID != "2AE-R2" {
		t.Errorf("находки: %+v", card.Anomalies)
	}
	if len(card.Requests) != 1 || card.Requests[0].ID != 7 {
		t.Errorf("заявки: %+v", card.Requests)
	}
	if len(card.Logins) != 1 || card.Forecast != nil {
		t.Errorf("логины %v, прогноз %+v", card.Logins, card.Forecast)
	}
}

func TestClientCardEdgeCases(t *testing.T) {
	h := fixtureRegistryAPI(t)
	get := func(key string) (clientCardPayload, int) {
		var card clientCardPayload
		code := getJSON(t, h.ClientCard, "GET /api/clients/{key}", "/api/clients/"+key, &card)
		return card, code
	}

	// Без абонента: код владельца из биллинга есть, в базе его нет.
	lone, code := get(loneINN)
	if code != http.StatusOK || lone.InBase || len(lone.Subscribers) != 1 || lone.Subscribers[0].InBase ||
		len(lone.Billing.Clients) != 1 || lone.Advice != nil || len(lone.ITS) != 0 {
		t.Errorf("без абонента (%d): %+v", code, lone)
	}

	// Без биллинга: организация из базы, второй КПП того же ИНН — соседняя карточка.
	idle, code := get(idleINN + "-" + processorKPP)
	if code != http.StatusOK || len(idle.Billing.Clients) != 0 || len(idle.Identifiers) != 0 ||
		len(idle.SameINN) != 1 || idle.SameINN[0].KPP != branchKPP || len(idle.Subscribers[0].Organizations) != 3 {
		t.Errorf("без биллинга (%d): %+v", code, idle)
	}

	// Нулевой ИНН: только свой идентификатор, без чужих находок и советов.
	zero, code := get("edo-2AE-Z2")
	if code != http.StatusOK || len(zero.Identifiers) != 1 || zero.Identifiers[0].EDOID != "2AE-Z2" ||
		len(zero.Anomalies) != 0 || len(zero.SameINN) != 0 || zero.Advice != nil || len(zero.Requests) != 0 {
		t.Errorf("нулевой ИНН (%d): %+v", code, zero)
	}

	// Голый ИНН с одним КПП открывает карточку; с двумя — нет: не гадаем.
	if card, code := get(processorINN); code != http.StatusOK || card.Key != processorINN+"-"+processorKPP {
		t.Errorf("по ИНН (%d): %+v", code, card.clientRef)
	}
	if _, code := get(idleINN); code != http.StatusNotFound {
		t.Errorf("ИНН с двумя КПП: код %d", code)
	}
	if _, code := get("nope"); code != http.StatusNotFound {
		t.Errorf("неизвестный ключ: код %d", code)
	}
}

func TestDashboardCountsAndLinks(t *testing.T) {
	h := fixtureRegistryAPI(t)
	var body dashboardPayload
	if code := getJSON(t, h.Dashboard, "GET /api/dashboard", "/api/dashboard", &body); code != http.StatusOK {
		t.Fatalf("код %d", code)
	}
	c := body.Counts
	if c.Invoice != 1 || c.LowRemainder != 1 || c.Renewals != 1 || c.Anomalies != 2 || c.Advice != 1 {
		t.Errorf("счётчики %+v", c)
	}
	if got := body.Billing.OverLimit[0].Clients; len(got) != 1 || got[0].Key != processorINN+"-"+processorKPP {
		t.Errorf("счёт ведёт не на ту карточку: %+v", got)
	}
	if got := body.Renewals.Items[0]; len(got.Clients) != 1 || got.Item.Renewal.TariffCode == "" {
		t.Errorf("продление: %+v", got)
	}
	// Тихий абонент с ЭДО без биллинга ведёт на все три свои организации;
	// нулевые ИНН не склеились с его организацией из нулей и остались «нет в базе».
	if len(body.Gaps.EDOWithoutBilling) != 1 || len(body.Gaps.EDOWithoutBilling[0].Clients) != 3 {
		t.Errorf("ЭДО без биллинга: %+v", body.Gaps.EDOWithoutBilling)
	}
	if c.NotInBase != 3 || body.Gaps.NotInBase[0].Clients[0].Key == "" {
		t.Errorf("нет в базе: %+v", body.Gaps.NotInBase)
	}
	if c.Drafts != 3 || c.Exported != 0 || len(body.Requests) != 3 || len(body.Requests[0].Clients) > 1 {
		t.Errorf("заявки %+v", body.Requests)
	}
	if body.Sources.Subscribers == "" || body.Sources.ITS == "" || body.Sources.EPDUsage == "" || body.Sources.Licenses != "" {
		t.Errorf("свежесть источников: %+v", body.Sources)
	}
	if code := getJSON(t, h.Dashboard, "GET /api/dashboard", "/api/dashboard?days=0", &body); code != http.StatusBadRequest {
		t.Errorf("days=0: код %d", code)
	}
}

// Абонент нашего клиента, которого нет в базе абонентов 1С, известен по нашему
// ЭДО: карточка помечена нашей, имя абонента — из отчёта трафика.
func TestClientCardOwnerKnownFromOurEDO(t *testing.T) {
	month := stubUsage{ok: true, rows: []partner.EDOTrafficRow{
		{EDOID: "2AE-NEW", INN: "7700000066", KPP: "770001001", ClientName: "ООО Новая связь",
			Subscriber: "CL-300: Новый абонент", SupportFrom: "2026-09-07"},
	}}
	h := fixtureRegistryAPI(t).WithForecast(month)
	var card clientCardPayload
	if code := getJSON(t, h.ClientCard, "GET /api/clients/{key}", "/api/clients/7700000066-770001001", &card); code != http.StatusOK {
		t.Fatalf("код %d", code)
	}
	if !card.Ours {
		t.Errorf("клиент не наш: %+v", card.clientRef)
	}
	if len(card.Subscribers) != 1 || card.Subscribers[0].Code != "CL-300" || card.Subscribers[0].InBase ||
		card.Subscribers[0].Name != "Новый абонент" {
		t.Errorf("абонент: %+v", card.Subscribers)
	}
}

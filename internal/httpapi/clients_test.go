package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"partnerops/internal/partner"
	"partnerops/internal/store"
)

// draftsStore отдаёт заранее заданные данные заявок, свежие сверху.
type draftsStore struct {
	RequestStore
	payloads []string
}

func (s draftsStore) Payloads(context.Context) ([]string, error) { return s.payloads, nil }

type registryClients []store.IdentifierRecord

func (c registryClients) All(context.Context) ([]store.IdentifierRecord, error) { return c, nil }

type cardJSON struct {
	Row struct {
		CompanyName string `json:"companyName"`
		INN         string `json:"inn"`
		KPP         string `json:"kpp"`
		Login       string `json:"login"`
		OwnerCode   string `json:"ownerCode"`
		RegNumber   string `json:"regNumber"`
		Responsible string `json:"responsible"`
		Phone       string `json:"phone"`
		Email       string `json:"email"`
		TariffCode  string `json:"tariffCode"`
		StartDate   string `json:"startDate"`
	} `json:"row"`
	Responsible string   `json:"responsible"`
	Email       string   `json:"email"`
	Source      string   `json:"source"`
	EDOIDs      []string `json:"edoIds"`
}

func fetchClients(t *testing.T, h *Requests) map[string]cardJSON {
	t.Helper()
	rec := httptest.NewRecorder()
	h.Clients(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/its/clients", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("код %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Clients []cardJSON `json:"clients"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("ответ не разбирается: %v", err)
	}
	byINN := map[string]cardJSON{}
	for _, card := range body.Clients {
		byINN[card.Row.INN] = card
	}
	return byINN
}

func TestClientsMergeLastRequestWithRegistry(t *testing.T) {
	drafts := draftsStore{payloads: []string{
		// Свежая заявка на «Ромашку»: её данные и должны попасть в карточку.
		`{"responsible":"Петров","email":"zakaz@partner.ru","rows":[{"inn":"7811000310","kpp":"780001001",
		  "companyName":"ООО Ромашка (старое)","regNumber":"18117482","responsible":"Иванов",
		  "phone":"1234567","email":"buh@romashka.ru","tariffCode":"2083","startDate":"01.10.26"}]}`,
		`{"responsible":"Сидоров","rows":[{"inn":"7811000310","kpp":"780001001","regNumber":"00000000"}]}`,
		`не JSON`,
	}}
	registry := registryClients{
		{EDOID: "2AE-1", INN: "7811000310", KPP: "780001001", ClientName: "ООО Ромашка", Login: "romashka", OwnerCode: "CL-1"},
		{EDOID: "2AE-2", INN: "500100000259", ClientName: "ИП Лисицын", OwnerCode: "FR-FR-7"},
	}

	cards := fetchClients(t, NewRequests(drafts, nil).WithClients(registry))

	if len(cards) != 2 {
		t.Fatalf("карточек %d, ожидали 2: %+v", len(cards), cards)
	}
	romashka := cards["7811000310"]
	if romashka.Row.RegNumber != "18117482" || romashka.Row.Responsible != "Иванов" ||
		romashka.Row.Phone != "1234567" || romashka.Row.Email != "buh@romashka.ru" {
		t.Errorf("поля последней заявки не перенесены: %+v", romashka.Row)
	}
	if romashka.Responsible != "Петров" || romashka.Email != "zakaz@partner.ru" {
		t.Errorf("шапка последней заявки: %q %q", romashka.Responsible, romashka.Email)
	}
	// Последняя заявка важнее реестра; чего в ней нет — из реестра.
	if romashka.Row.CompanyName != "ООО Ромашка (старое)" {
		t.Errorf("наименование не из последней заявки: %+v", romashka.Row)
	}
	if romashka.Row.Login != "romashka" || romashka.Row.OwnerCode != "CL-1" {
		t.Errorf("пустые в заявке поля не дополнены реестром: %+v", romashka.Row)
	}
	// Тариф — клиента (продлевают тот же); дата начала — свойство той заявки.
	if romashka.Row.TariffCode != "2083" || romashka.Row.StartDate != "" {
		t.Errorf("тариф и дата: %+v", romashka.Row)
	}
	if romashka.Source != "registry,request" {
		t.Errorf("источник %q", romashka.Source)
	}

	ip := cards["500100000259"]
	if ip.Row.CompanyName != "ИП Лисицын" || ip.Row.OwnerCode != "FR-FR-7" || ip.Source != "registry" {
		t.Errorf("клиент только из реестра: %+v", ip)
	}
}

// Логин и код владельца, введённые в последней заявке, важнее реестра.
func TestClientsLastRequestBeatsRegistry(t *testing.T) {
	drafts := draftsStore{payloads: []string{
		`{"rows":[{"inn":"7811000310","kpp":"780001001","login":"from-request","ownerCode":"CL-9",
		  "regNumber":"18117482","tariffCode":"2083","city":"Санкт-Петербург","workplaces":3}]}`,
	}}
	registry := registryClients{
		{EDOID: "2AE-1", INN: "7811000310", KPP: "780001001", ClientName: "ООО Ромашка", Login: "romashka", OwnerCode: "CL-1"},
	}
	card := fetchClients(t, NewRequests(drafts, nil).WithClients(registry))["7811000310"]
	if card.Row.Login != "from-request" || card.Row.OwnerCode != "CL-9" {
		t.Errorf("логин и владелец не из заявки: %+v", card.Row)
	}
	if card.Row.CompanyName != "ООО Ромашка" || card.Row.KPP != "780001001" ||
		card.Row.RegNumber != "18117482" || card.Row.TariffCode != "2083" {
		t.Errorf("поля клиента: %+v", card.Row)
	}
}

type subscriberBase []store.SubscriberRecord

func (s subscriberBase) All(context.Context) ([]store.SubscriberRecord, error) { return s, nil }

// Ответственный клиента — ФИО абонента-владельца из базы 1С, если в последней
// заявке его не вводили.
func TestClientsResponsibleFromSubscriber(t *testing.T) {
	registry := registryClients{
		{EDOID: "2AE-1", INN: "780700000416", ClientName: "ИП Пчёлкин", OwnerCode: "CL-1000764"},
		{EDOID: "2AE-2", INN: "7811000310", KPP: "780001001", ClientName: "ООО Ромашка", OwnerCode: "cl-1"},
	}
	subs := subscriberBase{
		{Code: "CL-1000764", Name: "Сидорук Глеб Викторович"},
		{Code: "CL-1", Name: "Петров Пётр"},
	}
	drafts := draftsStore{payloads: []string{
		`{"rows":[{"inn":"7811000310","kpp":"780001001","responsible":"Иванов"}]}`,
	}}
	cards := fetchClients(t, NewRequests(drafts, nil).WithClients(registry).WithSubscribers(subs))
	if got := cards["780700000416"].Row.Responsible; got != "Сидорук Глеб Викторович" {
		t.Errorf("ответственный из базы абонентов: %q", got)
	}
	if got := cards["7811000310"].Row.Responsible; got != "Иванов" {
		t.Errorf("ответственный последней заявки важнее базы абонентов: %q", got)
	}
}

// name абонента Фреша — почта или логин, у иных — номер или организация: это не ФИО.
func TestPersonName(t *testing.T) {
	for name, want := range map[string]bool{
		"Сидорук Глеб Викторович":  true,
		"Евгений Ковалев":          true,
		"Петрова-Водкина Анна":     true,
		"client-trucks@example.ru": false,
		"800000274":                false,
		"1С:Франчайзи Пример":      false,
		`ООО "Ромашка"`:            false,
		"Иванов":                   false,
		"vorota-78":                false,
	} {
		if got := personName(name); got != want {
			t.Errorf("personName(%q) = %v, want %v", name, got, want)
		}
	}
}

// Идентификаторы ЭДО организации — в её карточке, все и без повторов:
// по ним в форме заявки выбирают клиента.
func TestClientsCarryEDOIdentifiers(t *testing.T) {
	registry := registryClients{
		{EDOID: "2AE-B", INN: "7811000310", KPP: "780001001", ClientName: "ООО Ромашка", OwnerCode: "CL-1"},
		{EDOID: "2AE-A", INN: "7811000310", KPP: "780001001", ClientName: "ООО Ромашка", OwnerCode: "CL-1"},
		{EDOID: "2AE-A", INN: "7811000310", KPP: "780001001", ClientName: "ООО Ромашка", OwnerCode: "CL-1"},
		{EDOID: "2AE-C", INN: "500100000259", ClientName: "ИП Лисицын", OwnerCode: "CL-2"},
	}

	cards := fetchClients(t, NewRequests(draftsStore{}, nil).WithClients(registry))

	if got := strings.Join(cards["7811000310"].EDOIDs, ","); got != "2AE-A,2AE-B" {
		t.Errorf("идентификаторы Ромашки %q, ожидали 2AE-A,2AE-B", got)
	}
	if got := strings.Join(cards["500100000259"].EDOIDs, ","); got != "2AE-C" {
		t.Errorf("идентификаторы ИП %q, ожидали 2AE-C", got)
	}
}

// memoryPrograms — хранилище программ в памяти.
type memoryPrograms struct{ list []store.ClientProgram }

func (m *memoryPrograms) Replace(_ context.Context, inn, kpp string, programs []store.ClientProgram,
	at time.Time,
) error {
	kept := m.list[:0]
	for _, program := range m.list {
		if program.INN != inn || program.KPP != kpp {
			kept = append(kept, program)
		}
	}
	for _, program := range programs {
		program.CheckedAt = at
		kept = append(kept, program)
	}
	m.list = kept
	return nil
}

func (m *memoryPrograms) All(context.Context) ([]store.ClientProgram, error) { return m.list, nil }

// portalPrograms отвечает на проверку по логину и запоминает, чем искали.
type portalPrograms struct{ asked string }

func (p *portalPrograms) ProgramsByLogin(_ context.Context, login string) ([]partner.ProgramAccess, error) {
	p.asked = "login:" + login
	return []partner.ProgramAccess{
		{RegNumber: "200001912345", Program: "Бухгалтерия предприятия", HasAccess: true},
		{RegNumber: "200001967890", Program: "Зарплата и Управление Персоналом",
			MissingConditions: []string{"Договор 1С:ИТС"}},
	}, nil
}

func (p *portalPrograms) ProgramsByRegNumber(_ context.Context, regNumber string) ([]partner.ProgramAccess, error) {
	p.asked = "reg:" + regNumber
	return nil, nil
}

func TestCheckProgramsStoresResultInClientCard(t *testing.T) {
	programs := &memoryPrograms{}
	portal := &portalPrograms{}
	registry := registryClients{{EDOID: "2AE-1", INN: "7811000310", KPP: "780001001", ClientName: "ООО Ромашка", Login: "romashka"}}
	h := NewRequests(draftsStore{}, nil).WithClients(registry).WithPrograms(programs, portal)

	rec := httptest.NewRecorder()
	h.CheckPrograms(rec, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/its/clients/programs",
		strings.NewReader(`{"inn":"7811000310","kpp":"780001001","login":" buh@romashka.ru ","regNumber":"1"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("код %d: %s", rec.Code, rec.Body.String())
	}
	if portal.asked != "login:buh@romashka.ru" {
		t.Errorf("логин приоритетнее регномера, а искали %q", portal.asked)
	}

	rec = httptest.NewRecorder()
	h.Clients(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/its/clients", nil))
	var body struct {
		Clients []struct {
			Programs []struct {
				RegNumber string   `json:"regNumber"`
				HasAccess bool     `json:"hasAccess"`
				Missing   []string `json:"missing"`
			} `json:"programs"`
			ProgramsLogin     string     `json:"programsLogin"`
			ProgramsCheckedAt *time.Time `json:"programsCheckedAt"`
		} `json:"clients"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("ответ не разбирается: %v", err)
	}
	if len(body.Clients) != 1 || len(body.Clients[0].Programs) != 2 {
		t.Fatalf("программы не попали в карточку: %s", rec.Body.String())
	}
	card := body.Clients[0]
	if card.ProgramsLogin != "buh@romashka.ru" || card.ProgramsCheckedAt == nil ||
		card.Programs[1].RegNumber != "200001967890" || card.Programs[1].HasAccess ||
		len(card.Programs[1].Missing) != 1 {
		t.Errorf("карточка: %+v", card)
	}
}

func TestCheckProgramsNeedsLoginOrRegNumber(t *testing.T) {
	h := NewRequests(draftsStore{}, nil).WithPrograms(&memoryPrograms{}, &portalPrograms{})
	rec := httptest.NewRecorder()
	h.CheckPrograms(rec, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/its/clients/programs",
		strings.NewReader(`{"inn":"7811000310"}`)))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("код %d, ожидали 400", rec.Code)
	}
}

func TestCheckProgramsRejectsNonNumericRegNumber(t *testing.T) {
	portal := &portalPrograms{}
	h := NewRequests(draftsStore{}, nil).WithPrograms(&memoryPrograms{}, portal)
	rec := httptest.NewRecorder()
	h.CheckPrograms(rec, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/its/clients/programs",
		strings.NewReader(`{"inn":"7811000310","regNumber":"12a"}`)))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("код %d, ожидали 400: опечатка в регномере — не сбой 1С", rec.Code)
	}
	if portal.asked != "" {
		t.Errorf("в 1С ушёл запрос с негодным регномером: %q", portal.asked)
	}
}

// Подбор — только наши клиенты: заявка на чужую организацию и ушедший из
// биллинга идентификатор клиента не дают; привязанный после закрытого
// биллинга идентификатор приходит из отчёта трафика.
func TestClientsOfferOnlyOurClients(t *testing.T) {
	drafts := draftsStore{payloads: []string{`{"rows":[{"inn":"7700000009","companyName":"ООО Чужой"}]}`}}
	registry := registryClients{
		{EDOID: "2AE-1", INN: "7811000310", KPP: "780001001", ClientName: "ООО Ромашка", LastPeriod: "2026-08"},
		{EDOID: "2AE-OLD", INN: "7700000044", KPP: "770001001", ClientName: "ООО Ушедший", LastPeriod: "2026-05"},
	}
	traffic := stubUsage{rows: []partner.EDOTrafficRow{{EDOID: "2AE-NEW", INN: "4200000320", KPP: "420001001",
		ClientName: `ООО "Центр"`, Subscriber: "CL-7000582: Иванов", Login: "center"}}}

	cards := fetchClients(t, NewRequests(drafts, nil).WithClients(registry, traffic))

	if len(cards) != 2 {
		t.Fatalf("карточек %d, ожидали 2 наших: %+v", len(cards), cards)
	}
	if _, ok := cards["7811000310"]; !ok {
		t.Errorf("нет клиента из биллинга: %+v", cards)
	}
	center := cards["4200000320"]
	if center.Row.OwnerCode != "CL-7000582" || center.Row.Login != "center" || center.Row.KPP != "420001001" {
		t.Errorf("клиент из отчёта трафика: %+v", center)
	}
}

// Код владельца и логин есть не у каждого идентификатора: второй бывает не
// привязан к абоненту, в отчёте трафика нет логина. Подбор берёт их у других
// идентификаторов той же организации, в том числе ушедших из биллинга.
func TestClientsFillOwnerAndLoginFromSameOrganization(t *testing.T) {
	registry := registryClients{
		{EDOID: "2AE-OLD", INN: "4200000320", KPP: "420001001", ClientName: `ООО "Центр переработки"`,
			Login: "v.melnichuk@plantcenter.example", OwnerCode: "CL-7000582", LastPeriod: "2026-05"},
		{EDOID: "2AE-CUR", INN: "4200000320", KPP: "420001001", ClientName: `ООО "Центр переработки"`, LastPeriod: "2026-08"},
		{EDOID: "2AE-GONE", INN: "7811000310", KPP: "780001001", Login: "romashka", OwnerCode: "CL-1", LastPeriod: "2026-05"},
	}
	traffic := stubUsage{rows: []partner.EDOTrafficRow{{EDOID: "2AE-NEW", INN: "7811000310", KPP: "780001001",
		ClientName: "ООО Ромашка"}}}

	cards := fetchClients(t, NewRequests(draftsStore{}, nil).WithClients(registry, traffic))

	if center := cards["4200000320"]; center.Row.OwnerCode != "CL-7000582" || center.Row.Login != "v.melnichuk@plantcenter.example" {
		t.Errorf("идентификатор без владельца: %+v", center.Row)
	}
	if romashka := cards["7811000310"]; romashka.Row.OwnerCode != "CL-1" || romashka.Row.Login != "romashka" {
		t.Errorf("идентификатор из отчёта трафика: %+v", romashka.Row)
	}
}

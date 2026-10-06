package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"partnerops/internal/itsreq"
	"partnerops/internal/partner"
	"partnerops/internal/store"
)

// ClientSource — реестр клиентов из снимков ЭДО: то, что видно на экране биллинга.
type ClientSource interface {
	All(ctx context.Context) ([]store.IdentifierRecord, error)
}

// WithClients включает подбор наших клиентов 1С-ЭДО в форме заявки: реестр ЭДО
// и отчёты трафика (см. ourEDOIDs). Без него подбора нет, а сверка с 1С знает
// только базу абонентов.
func (h *Requests) WithClients(clients ClientSource, traffic ...EPDUsageReader) *Requests {
	h.clients, h.traffic = clients, traffic
	return h
}

// SubscriberNames — сохранённая база абонентов 1С (/rest/public/subscriber);
// *store.Subscribers ей удовлетворяет.
type SubscriberNames interface {
	All(ctx context.Context) ([]store.SubscriberRecord, error)
}

// WithSubscribers включает ответственного клиента из базы абонентов: name
// абонента-владельца идентификатора ЭДО — его ФИО (поле «ФИО» карточки абонента
// на портале 1С-ЭДО). Логина и почты абонента этот метод API не отдаёт.
func (h *Requests) WithSubscribers(subscribers SubscriberNames) *Requests {
	h.subscribers = subscribers
	return h
}

// subscriberNames — ФИО абонентов по коду (CL-…), без учёта регистра.
func (h *Requests) subscriberNames(ctx context.Context) map[string]string {
	if h.subscribers == nil {
		return nil
	}
	list, err := h.subscribers.All(ctx)
	if err != nil {
		// Без ФИО подбор работает как раньше: ответственного вводят руками.
		slog.Warn("база абонентов для подбора клиента не прочитана", "err", err)
		return nil
	}
	names := make(map[string]string, len(list))
	for _, s := range list {
		if name := strings.TrimSpace(s.Name); personName(name) {
			names[strings.ToUpper(strings.TrimSpace(s.Code))] = name
		}
	}
	return names
}

// personName отличает ФИО от прочего, что 1С кладёт в name абонента: у
// абонентов Фреша там почта или логин, у иных — номер или название
// организации. ФИО — два-три слова из одних букв (дефис допустим).
func personName(name string) bool {
	words := strings.Fields(name)
	if len(words) < 2 || len(words) > 4 {
		return false
	}
	for _, word := range words {
		for _, r := range word {
			if !unicode.IsLetter(r) && r != '-' && r != '.' {
				return false
			}
		}
	}
	return true
}

// ourClients — наши идентификаторы ЭДО с реквизитами: из реестра и отчётов трафика.
func (h *Requests) ourClients(ctx context.Context) ([]ourIdentifier, error) {
	if h.clients == nil {
		return nil, nil
	}
	ids, err := h.clients.All(ctx)
	if err != nil {
		return nil, err
	}
	var traffic []partner.EDOTrafficRow
	for _, reader := range h.traffic {
		rows, err := reader.Rows(ctx)
		if err != nil {
			return nil, err
		}
		traffic = append(traffic, rows...)
	}
	var imported []partner.EPDBillingRow
	if h.epdImport != nil {
		if imported, err = h.epdImport.Rows(ctx); err != nil {
			return nil, err
		}
	}
	return ourIdentifiers(ids, traffic, imported, time.Now().In(moscow).Format(time.DateOnly)), nil
}

// addSource дописывает источник карточки: "registry", "registry,request".
func addSource(c *clientCard, source string) {
	if c.Source == "" {
		c.Source = source
		return
	}
	if !slices.Contains(strings.Split(c.Source, ","), source) {
		c.Source += "," + source
	}
}

// ProgramStore хранит последнюю проверку условий сопровождения по клиенту.
type ProgramStore interface {
	Replace(ctx context.Context, inn, kpp string, programs []store.ClientProgram, at time.Time) error
	All(ctx context.Context) ([]store.ClientProgram, error)
}

// ProgramChecker — проверка условий сопровождения в партнёрском API 1С;
// *partner.Client ей удовлетворяет.
type ProgramChecker interface {
	ProgramsByLogin(ctx context.Context, login string) ([]partner.ProgramAccess, error)
	ProgramsByRegNumber(ctx context.Context, regNumber string) ([]partner.ProgramAccess, error)
}

// WithPrograms включает программы клиента: хранение и проверку в 1С.
// checker nil — проверка недоступна, сохранённые программы всё равно видны.
func (h *Requests) WithPrograms(programs ProgramStore, checker ProgramChecker) *Requests {
	h.programs, h.checker = programs, checker
	return h
}

// clientProgram — строка «Проверки условий сопровождения» Личного кабинета.
type clientProgram struct {
	RegNumber string `json:"regNumber"`
	Program   string `json:"program"`
	HasAccess bool   `json:"hasAccess"`
	// Missing — недостающие условия сопровождения; пусто, если выполнены.
	Missing []string `json:"missing,omitempty"`
}

// clientCard — всё, что известно о клиенте, в полях заявки.
//
// Row — поля строки таблицы; пустое значение означает «неизвестно», форма
// его не подставляет. Responsible и Email — шапка последней заявки на клиента.
type clientCard struct {
	Row         itsreq.Row `json:"row"`
	Responsible string     `json:"responsible,omitempty"`
	Email       string     `json:"email,omitempty"`
	// ITSTariffs — «Тарифы ИТС» клиента из реестра ЭДО или выгрузки биллинга ЭПД.
	ITSTariffs string `json:"itsTariffs,omitempty"`
	// Source — откуда карточка: "registry" (наш клиент по реестру ЭДО и отчётам
	// трафика), с прошлой заявкой — "registry,request".
	Source string `json:"source"`
	// EDOIDs — идентификаторы участника ЭДО организации (ИНН+КПП) по реестру и
	// отчётам трафика. В файл заявки не попадают: по ним выбирают клиента и
	// сверяют, к какому логину привязан идентификатор.
	EDOIDs []string `json:"edoIds"`
	// Programs — программы Личного кабинета по последней проверке в 1С.
	Programs          []clientProgram `json:"programs,omitempty"`
	ProgramsLogin     string          `json:"programsLogin,omitempty"`
	ProgramsCheckedAt *time.Time      `json:"programsCheckedAt,omitempty"`
}

// Clients отдаёт карточки наших клиентов 1С-ЭДО для автозаполнения заявки.
//
// Клиенты — только наши (реестр ЭДО и отчёты трафика): база абонентов 1С
// держит и чужие организации агрегаторов, им в подборе не место. Прошлые
// заявки дополняют карточку всем, что туда вводили о клиенте: регномер,
// тариф, адрес, телефон, ответственного, логин, код владельца. Приоритет поля:
// последняя заявка, затем данные API (реестр ЭДО, отчёты трафика, база
// абонентов), затем загруженная выгрузка биллинга ЭПД, иначе пусто.
func (h *Requests) Clients(w http.ResponseWriter, r *http.Request) {
	ours, err := h.ourClients(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось прочитать реестр клиентов.")
		return
	}
	cards := map[string]*clientCard{}
	var order []string
	for _, id := range ours {
		inn := strings.TrimSpace(id.INN)
		if inn == "" {
			continue
		}
		kpp := strings.TrimSpace(id.KPP)
		key := inn + "/" + kpp
		c, ok := cards[key]
		if !ok {
			c = &clientCard{Row: itsreq.Row{INN: inn, KPP: kpp}, Source: "registry"}
			cards[key] = c
			order = append(order, key)
		}
		c.Row.CompanyName = firstNonEmpty(c.Row.CompanyName, id.ClientName)
		c.Row.Login = firstNonEmpty(c.Row.Login, id.Login)
		c.Row.OwnerCode = firstNonEmpty(c.Row.OwnerCode, id.OwnerCode)
		c.Row.Responsible = firstNonEmpty(c.Row.Responsible, id.Responsible)
		c.ITSTariffs = firstNonEmpty(c.ITSTariffs, id.ITSTariffs)
		if edoID := strings.TrimSpace(id.EDOID); edoID != "" && !slices.Contains(c.EDOIDs, edoID) {
			c.EDOIDs = append(c.EDOIDs, edoID)
		}
	}

	if names := h.subscriberNames(r.Context()); names != nil {
		for _, c := range cards {
			// ФИО из базы абонентов 1С важнее ФИО из ручной выгрузки.
			c.Row.Responsible = firstNonEmpty(names[strings.ToUpper(c.Row.OwnerCode)], c.Row.Responsible)
		}
	}

	payloads, err := h.store.Payloads(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось прочитать заявки.")
		return
	}
	for _, payload := range payloads {
		var request itsreq.Request
		if err := json.Unmarshal([]byte(payload), &request); err != nil {
			// Один битый черновик не повод лишать подбора по остальным.
			slog.Warn("черновик заявки не разобрать", "err", err)
			continue
		}
		for _, row := range request.Rows {
			c, ok := cards[strings.TrimSpace(row.INN)+"/"+strings.TrimSpace(row.KPP)]
			if !ok || strings.Contains(c.Source, "request") {
				continue // не наш клиент; или заявки идут свежими сверху: первая и есть последняя
			}
			fields := clientFields(row)
			fields.INN, fields.KPP = c.Row.INN, c.Row.KPP
			// Последняя заявка важнее реестра: в неё вводили руками и проверяли.
			// Реестр заполняет только то, чего в заявке нет.
			fields.CompanyName = firstNonEmpty(fields.CompanyName, c.Row.CompanyName)
			fields.Login = firstNonEmpty(fields.Login, c.Row.Login)
			fields.OwnerCode = firstNonEmpty(fields.OwnerCode, c.Row.OwnerCode)
			fields.Responsible = firstNonEmpty(fields.Responsible, c.Row.Responsible)
			c.Row = fields
			c.Responsible, c.Email = request.Responsible, request.Email
			addSource(c, "request")
		}
	}

	if h.programs != nil {
		programs, err := h.programs.All(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "Не удалось прочитать программы клиентов.")
			return
		}
		for _, program := range programs {
			c, ok := cards[program.INN+"/"+program.KPP]
			if !ok {
				continue // не наш клиент — подбирать нечего
			}
			c.Programs = append(c.Programs, programJSON(program))
			c.ProgramsLogin = program.Login
			checked := program.CheckedAt
			c.ProgramsCheckedAt = &checked
		}
	}

	list := make([]clientCard, 0, len(order))
	for _, key := range order {
		card := cards[key]
		if card.EDOIDs == nil {
			card.EDOIDs = []string{}
		}
		slices.Sort(card.EDOIDs)
		list = append(list, *card)
	}
	sort.SliceStable(list, func(i, j int) bool {
		return strings.ToLower(list[i].Row.CompanyName) < strings.ToLower(list[j].Row.CompanyName)
	})
	writeJSON(w, http.StatusOK, map[string]any{"clients": list})
}
func programJSON(program store.ClientProgram) clientProgram {
	return clientProgram{RegNumber: program.RegNumber, Program: program.Program,
		HasAccess: program.HasAccess, Missing: program.Missing}
}

// CheckPrograms проверяет условия сопровождения программ клиента в 1С — как
// страница портала «Проверка условий сопровождения» — и запоминает ответ
// в карточке клиента. Ищет по логину, без него — по регномеру.
func (h *Requests) CheckPrograms(w http.ResponseWriter, r *http.Request) {
	if h.programs == nil || h.checker == nil {
		writeError(w, http.StatusServiceUnavailable, "not_configured", "Проверка в 1С не настроена.")
		return
	}
	var body struct {
		INN       string `json:"inn"`
		KPP       string `json:"kpp"`
		Login     string `json:"login"`
		RegNumber string `json:"regNumber"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Неверный формат запроса.")
		return
	}
	inn, kpp := strings.TrimSpace(body.INN), strings.TrimSpace(body.KPP)
	login, regNumber := strings.TrimSpace(body.Login), strings.TrimSpace(body.RegNumber)
	if inn == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "Укажите ИНН клиента: программы хранятся в его карточке.")
		return
	}

	var found []partner.ProgramAccess
	var err error
	switch {
	case login != "":
		found, err = h.checker.ProgramsByLogin(r.Context(), login)
	case regNumber != "":
		// Опечатка в регномере — ошибка ввода, а не сбой 1С: до API её не пускаем.
		if _, parseErr := strconv.ParseUint(regNumber, 10, 63); parseErr != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "Регистрационный номер — только цифры.")
			return
		}
		found, err = h.checker.ProgramsByRegNumber(r.Context(), regNumber)
	default:
		writeError(w, http.StatusBadRequest, "bad_request", "Укажите логин Личного кабинета или регистрационный номер.")
		return
	}
	if err != nil {
		slog.Warn("условия сопровождения не проверены", "err", err)
		message := "1С не ответила на проверку условий сопровождения. Попробуйте позже."
		if partner.IsUnauthorized(err) {
			message = "1С не приняла логин и пароль партнёрского API."
		}
		writeError(w, http.StatusBadGateway, "portal_failed", message)
		return
	}

	at := time.Now().UTC()
	rows := make([]store.ClientProgram, 0, len(found))
	list := make([]clientProgram, 0, len(found))
	for _, program := range found {
		row := store.ClientProgram{INN: inn, KPP: kpp, Login: login, RegNumber: program.RegNumber,
			Program: program.Program, HasAccess: program.HasAccess, Missing: program.MissingConditions}
		rows = append(rows, row)
		list = append(list, programJSON(row))
	}
	if err := h.programs.Replace(r.Context(), inn, kpp, rows, at); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось сохранить программы клиента.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"programs": list, "checkedAt": at})
}

// clientFields оставляет от строки прошлой заявки сведения о клиенте и его
// тариф (вид ИТС обычно продлевают тот же). Даты и операция относятся к той
// заявке, а не к клиенту.
func clientFields(row itsreq.Row) itsreq.Row {
	return itsreq.Row{
		TariffCode: row.TariffCode, RegNumber: row.RegNumber, CompanyName: row.CompanyName,
		INN: strings.TrimSpace(row.INN), KPP: strings.TrimSpace(row.KPP),
		Workplaces: row.Workplaces, ActivityType: row.ActivityType,
		Director: row.Director, Responsible: row.Responsible,
		PostalCode: row.PostalCode, City: row.City, Street: row.Street,
		House: row.House, Building: row.Building, Flat: row.Flat,
		PhoneCode: row.PhoneCode, Phone: row.Phone, Fax: row.Fax, Email: row.Email,
		DeliveryType: row.DeliveryType, DistributorCode: row.DistributorCode,
		ExtraRegNumbers: row.ExtraRegNumbers,
		OwnerCode:       row.OwnerCode, Login: row.Login,
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

// checkDirectory — справочник для сверки заявки: база абонентов 1С дополнена
// нашими клиентами 1С-ЭДО. /rest/public/subscriber отдаёт не всех абонентов,
// чьи идентификаторы есть в нашем биллинге (живой пример — владелец CL-7000582
// с ИНН 4200000320), и без дополнения сверка объявила бы нашего клиента чужим.
func (h *Requests) checkDirectory() itsreq.Directory {
	if h.directory == nil {
		return nil
	}
	if h.clients == nil {
		return h.directory
	}
	return ourDirectory{Directory: h.directory, ours: h.ourClients}
}

type ourDirectory struct {
	itsreq.Directory
	ours func(ctx context.Context) ([]ourIdentifier, error)
}

// Subscribers — база абонентов 1С и наши клиенты. Реестр не прочитался — сверка
// идёт по одной базе 1С, как без реестра.
func (d ourDirectory) Subscribers(ctx context.Context) ([]itsreq.Subscriber, error) {
	subs, err := d.Directory.Subscribers(ctx)
	if err != nil {
		return nil, err
	}
	ours, err := d.ours(ctx)
	if err != nil {
		slog.Warn("наши клиенты для сверки заявки не прочитаны", "err", err)
		return subs, nil
	}
	return withOurClients(subs, ours), nil
}

// withOurClients дописывает наших клиентов к базе абонентов, не трогая её
// (справочник кэширует ответ 1С): организацию — к абоненту-владельцу
// идентификатора, неизвестного владельца — новым абонентом. Идентификатор без
// владельца (в Кабинете партнёра не привязан к абоненту) даёт организацию
// абоненту без кода, если её ИНН нет больше нигде: ИНН наш, а код владельца
// сверка у такой организации не подскажет.
func withOurClients(subs []itsreq.Subscriber, ours []ourIdentifier) []itsreq.Subscriber {
	list := slices.Clone(subs)
	at := map[string]int{}
	for i, s := range subs {
		list[i].Organizations = slices.Clone(s.Organizations)
		at[strings.ToUpper(s.Code)] = i
	}
	add := func(i int, id ourIdentifier) {
		org := itsreq.Organization{Name: id.ClientName, INN: id.INN, KPP: id.KPP}
		for _, known := range list[i].Organizations {
			if known.INN == org.INN && known.KPP == org.KPP {
				return
			}
		}
		list[i].Organizations = append(list[i].Organizations, org)
	}
	var orphans []ourIdentifier
	for _, id := range ours {
		switch {
		case !validINN(id.INN):
		case id.OwnerCode == "":
			orphans = append(orphans, id)
		default:
			i, ok := at[strings.ToUpper(id.OwnerCode)]
			if !ok {
				i = len(list)
				at[strings.ToUpper(id.OwnerCode)] = i
				list = append(list, itsreq.Subscriber{Code: id.OwnerCode})
			}
			add(i, id)
		}
	}
	known := map[string]bool{}
	for _, s := range list {
		for _, org := range s.Organizations {
			known[org.INN] = true
		}
	}
	unowned := -1
	for _, id := range orphans {
		if known[id.INN] {
			continue
		}
		if unowned < 0 {
			unowned = len(list)
			list = append(list, itsreq.Subscriber{})
		}
		add(unowned, id)
	}
	return list
}

package httpapi

import (
	"fmt"
	"hash/fnv"
	"slices"
	"sort"
	"strings"

	"partnerops/internal/itsreq"
	"partnerops/internal/partner"
	"partnerops/internal/store"
)

// Ключ карточки клиента.
//
// Клиент — организация: ИНН+КПП. Так её видят все источники: реестр ЭДО
// (у одной организации бывает несколько идентификаторов — они в одной
// карточке), база абонентов (у абонента много организаций — абонент лишь
// группа сверху, в карточке его код и соседи), заявки, программы, совет тарифа.
// Абонента ключом не берём: организации без абонента («нет в базе») тоже
// клиенты, а один абонент-агрегатор (облако Фреш) держит сотни чужих друг
// другу ИП. Разные КПП одного ИНН — разные карточки (обособленные
// подразделения ведут свой ЭДО и свои договоры), связаны ссылкой «соседи».
//
// Формат: «ИНН-КПП», у ИП и организаций без КПП — «ИНН». Ключ читается
// человеком и собирается на фронте из тех же реквизитов (web/src/domain.ts
// clientKey), поэтому ссылки на карточку ставятся с любого экрана без запроса.
//
// Настоящего ИНН бывает нет: пусто или «0000000000» — 1С так заполняет
// организацию, которую завели по одному e-mail. Такие записи никогда не
// склеиваются между собой: идентификатор ЭДО получает ключ «edo-<ID>»,
// организация абонента — «sub-<код>-<хэш названия>». Склейка по нулевому ИНН
// слила бы в одну карточку чужие компании.
func clientKey(inn, kpp string) string {
	inn, kpp = strings.TrimSpace(inn), strings.TrimSpace(kpp)
	if !validINN(inn) {
		return ""
	}
	if kpp == "" || allZeros(kpp) {
		return inn
	}
	return inn + "-" + kpp
}

// validINN — 10 цифр у организации, 12 у ИП, и не одни нули.
func validINN(inn string) bool {
	if len(inn) != 10 && len(inn) != 12 {
		return false
	}
	for _, c := range inn {
		if c < '0' || c > '9' {
			return false
		}
	}
	return !allZeros(inn)
}

func allZeros(s string) bool {
	return strings.Trim(s, "0") == ""
}

// orphanOrgKey — ключ организации абонента без настоящего ИНН: стабилен, пока
// у неё не меняются название и реквизиты в 1С.
func orphanOrgKey(code, name, inn, kpp string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(strings.TrimSpace(name) + "|" + strings.TrimSpace(inn) + "|" + strings.TrimSpace(kpp)))
	return fmt.Sprintf("sub-%s-%08x", code, h.Sum32())
}

// clientRef — клиент в ссылке: ключ карточки и реквизиты.
type clientRef struct {
	Key        string `json:"key"`
	ClientName string `json:"clientName"`
	INN        string `json:"inn"`
	KPP        string `json:"kpp"`
}

// Источники клиента.
const (
	sourceRegistry   = "registry"
	sourceSubscriber = "subscriber"
	sourceRequest    = "request"
	sourceTraffic    = "traffic"
	// sourceImport — выгрузка «Детализация биллинга» ЭПД, загруженная руками.
	sourceImport = "import"
)

// clientEntry — всё, что связывает организацию с данными других экранов.
type clientEntry struct {
	clientRef
	// SubscriberCodes — абоненты: из базы абонентов и коды владельцев из биллинга.
	SubscriberCodes []string
	EDOIDs          []string
	Logins          []string
	Sources         []string
	// InBase — организация есть в базе абонентов 1С.
	InBase bool
	// Ours — наш клиент 1С-ЭДО: у него есть наш идентификатор ЭДО, см. ourIdentifiers.
	Ours bool
}

func (e *clientEntry) addSource(source string) { e.Sources = appendUniq(e.Sources, source) }

// clientIndex — клиенты и способы найти клиента по данным других экранов.
type clientIndex struct {
	list         []*clientEntry
	byKey        map[string]*clientEntry
	byEDO        map[string]*clientEntry
	byINN        map[string][]*clientEntry
	bySubscriber map[string][]*clientEntry
	// ownerNames — имя абонента-владельца по коду из реестра ЭДО и отчётов
	// трафика: для абонентов, которых нет в базе абонентов 1С.
	ownerNames map[string]string
}

// ourEDOIDs — идентификаторы ЭДО наших клиентов 1С-ЭДО по данным партнёрского API.
//
// API отдаёт только идентификаторы, привязанные к нашему коду партнёра, поэтому
// наш — любой идентификатор последнего снятого биллинга (в реестре это строки
// с самым поздним LastPeriod) или отчётов трафика: текущего месяца и года —
// они видят и связи, созданные после закрытого месяца биллинга. Не наши —
// ушедшие из биллинга и те, чьё сопровождение по отчёту трафика кончилось.
// Идентификатор без трафика, привязанный после закрытого месяца, API покажет
// только в следующем биллинге. today — ГГГГ-ММ-ДД, как даты отчёта трафика.
func ourEDOIDs(ids []store.IdentifierRecord, traffic []partner.EDOTrafficRow, today string) map[string]bool {
	latest := ""
	for _, id := range ids {
		latest = max(latest, id.LastPeriod)
	}
	ours := map[string]bool{}
	for _, id := range ids {
		if id.LastPeriod == latest {
			ours[id.EDOID] = true
		}
	}
	active, ended := map[string]bool{}, map[string]bool{}
	for _, row := range traffic {
		id := strings.TrimSpace(row.EDOID)
		if id == "" {
			continue
		}
		if to := strings.TrimSpace(row.SupportTo); to != "" && to < today {
			ended[id] = true
		} else {
			active[id] = true
		}
	}
	for id := range active {
		ours[id] = true
	}
	for id := range ended {
		if !active[id] {
			delete(ours, id)
		}
	}
	return ours
}

// ourIdentifier — наш идентификатор ЭДО с реквизитами клиента.
type ourIdentifier struct {
	EDOID      string
	INN        string
	KPP        string
	ClientName string
	Login      string
	OwnerCode  string
	ITSTariffs string
	// Responsible — ФИО абонента-владельца из выгрузки биллинга ЭПД; пусто,
	// если там почта или логин.
	Responsible string
}

// ourIdentifiers — наши идентификаторы (ourEDOIDs) с реквизитами: из реестра,
// привязанные после закрытого биллинга — из отчётов трафика, недостающие в API
// клиенты ЭПД — из загруженной выгрузки биллинга ЭПД. Выгрузка дополняет данные
// API только пустыми полями: API свежее ручного файла.
func ourIdentifiers(ids []store.IdentifierRecord, traffic []partner.EDOTrafficRow,
	imported []partner.EPDBillingRow, today string,
) []ourIdentifier {
	ours := ourEDOIDs(ids, traffic, today)
	at := map[string]int{}
	var list []ourIdentifier
	for _, id := range ids {
		if _, seen := at[id.EDOID]; ours[id.EDOID] && !seen {
			at[id.EDOID] = len(list)
			list = append(list, ourIdentifier{EDOID: id.EDOID, INN: strings.TrimSpace(id.INN),
				KPP: strings.TrimSpace(id.KPP), ClientName: strings.TrimSpace(id.ClientName),
				Login: strings.TrimSpace(id.Login), OwnerCode: strings.TrimSpace(id.OwnerCode),
				ITSTariffs: strings.TrimSpace(id.ITSTariffs)})
		}
	}
	for _, row := range traffic {
		id := strings.TrimSpace(row.EDOID)
		if _, seen := at[id]; ours[id] && !seen {
			at[id] = len(list)
			code, _ := partner.ParseOwner(row.Subscriber)
			list = append(list, ourIdentifier{EDOID: id, INN: strings.TrimSpace(row.INN),
				KPP: strings.TrimSpace(row.KPP), ClientName: strings.TrimSpace(row.ClientName),
				Login: strings.TrimSpace(row.Login), OwnerCode: code})
		}
	}
	for _, row := range imported {
		id := strings.TrimSpace(row.EDOID)
		if id == "" {
			continue
		}
		responsible := ""
		if contact := strings.TrimSpace(row.OwnerContact); personName(contact) {
			responsible = contact
		}
		i, seen := at[id]
		if !seen {
			at[id] = len(list)
			list = append(list, ourIdentifier{EDOID: id, INN: strings.TrimSpace(row.INN),
				KPP: strings.TrimSpace(row.KPP), ClientName: strings.TrimSpace(row.ClientName),
				Login: strings.TrimSpace(row.Login), OwnerCode: strings.TrimSpace(row.OwnerCode),
				ITSTariffs: strings.TrimSpace(row.ITSTariffs), Responsible: responsible})
			continue
		}
		known := &list[i]
		known.ClientName = firstNonEmpty(known.ClientName, row.ClientName)
		known.Login = firstNonEmpty(known.Login, row.Login)
		known.OwnerCode = firstNonEmpty(known.OwnerCode, row.OwnerCode)
		known.ITSTariffs = firstNonEmpty(known.ITSTariffs, row.ITSTariffs)
		known.Responsible = firstNonEmpty(known.Responsible, responsible)
	}
	fillOwnerAndLogin(list, ids, traffic)
	return list
}

// fillOwnerAndLogin дописывает нашим идентификаторам недостающие код владельца
// и логин по другим идентификаторам той же организации (ИНН и КПП). Они есть не
// у каждого: второй идентификатор клиента бывает не привязан к абоненту, в
// отчёте трафика нет логина. Берём и ушедшие из биллинга идентификаторы:
// организация та же — абонент и логин те же, а без них форма заявки не
// заполнит код владельца и логин нашему клиенту.
func fillOwnerAndLogin(list []ourIdentifier, ids []store.IdentifierRecord, traffic []partner.EDOTrafficRow) {
	owners, logins := map[string]string{}, map[string]string{}
	remember := func(inn, kpp, owner, login string) {
		key := clientKey(inn, kpp)
		if key == "" {
			return
		}
		owners[key] = firstNonEmpty(owners[key], strings.TrimSpace(owner))
		logins[key] = firstNonEmpty(logins[key], strings.TrimSpace(login))
	}
	for _, id := range list {
		remember(id.INN, id.KPP, id.OwnerCode, id.Login)
	}
	for _, id := range ids {
		remember(id.INN, id.KPP, id.OwnerCode, id.Login)
	}
	for _, row := range traffic {
		code, _ := partner.ParseOwner(row.Subscriber)
		remember(row.INN, row.KPP, code, row.Login)
	}
	for i := range list {
		key := clientKey(list[i].INN, list[i].KPP)
		if key == "" {
			continue
		}
		list[i].OwnerCode = firstNonEmpty(list[i].OwnerCode, owners[key])
		list[i].Login = firstNonEmpty(list[i].Login, logins[key])
	}
}

// buildClientIndex собирает клиентов из реестра ЭДО, отчётов трафика, базы
// абонентов и строк заявок. Название берётся в том же порядке: реестр свежее
// трафика, трафик — базы, база — заявок. Наши клиенты — см. ourEDOIDs; today
// (ГГГГ-ММ-ДД) отделяет действующее сопровождение от кончившегося.
//
// imported — загруженная выгрузка биллинга ЭПД: её клиенты наши (это наш
// биллинг), но реквизиты она лишь дополняет — реестр и трафик идут первыми.
func buildClientIndex(ids []store.IdentifierRecord, traffic []partner.EDOTrafficRow,
	imported []partner.EPDBillingRow, subs []store.SubscriberRecord, requestRows []itsreq.Row, today string,
) *clientIndex {
	x := &clientIndex{byKey: map[string]*clientEntry{}, byEDO: map[string]*clientEntry{},
		byINN: map[string][]*clientEntry{}, bySubscriber: map[string][]*clientEntry{}, ownerNames: map[string]string{}}
	ownerName := func(code, raw string) {
		if _, name := partner.ParseOwner(raw); code != "" {
			x.ownerNames[code] = firstNonEmpty(x.ownerNames[code], name)
		}
	}
	entry := func(key, inn, kpp string) *clientEntry {
		if e, ok := x.byKey[key]; ok {
			return e
		}
		e := &clientEntry{Key: key, INN: inn, KPP: kpp}
		x.byKey[key] = e
		x.list = append(x.list, e)
		if validINN(inn) {
			x.byINN[inn] = append(x.byINN[inn], e)
		}
		return e
	}
	ours := ourEDOIDs(ids, traffic, today)

	for _, id := range ids {
		inn, kpp := strings.TrimSpace(id.INN), strings.TrimSpace(id.KPP)
		key := clientKey(inn, kpp)
		if key == "" {
			key = "edo-" + id.EDOID
		}
		e := entry(key, inn, kpp)
		e.ClientName = firstNonEmpty(e.ClientName, id.ClientName)
		e.EDOIDs = appendUniq(e.EDOIDs, id.EDOID)
		e.Logins = appendUniq(e.Logins, strings.TrimSpace(id.Login))
		e.SubscriberCodes = appendUniq(e.SubscriberCodes, strings.TrimSpace(id.OwnerCode))
		ownerName(strings.TrimSpace(id.OwnerCode), id.OwnerRaw)
		e.addSource(sourceRegistry)
		e.Ours = e.Ours || ours[id.EDOID]
		x.byEDO[id.EDOID] = e
	}

	// Отчёты трафика видят связи, которых в закрытом биллинге ещё нет.
	for _, row := range traffic {
		id := strings.TrimSpace(row.EDOID)
		if !ours[id] {
			continue
		}
		e, ok := x.byEDO[id]
		if !ok {
			inn, kpp := strings.TrimSpace(row.INN), strings.TrimSpace(row.KPP)
			key := clientKey(inn, kpp)
			if key == "" {
				key = "edo-" + id
			}
			e = entry(key, inn, kpp)
			e.EDOIDs = appendUniq(e.EDOIDs, id)
			x.byEDO[id] = e
		}
		code, _ := partner.ParseOwner(row.Subscriber)
		e.ClientName = firstNonEmpty(e.ClientName, row.ClientName)
		e.Logins = appendUniq(e.Logins, strings.TrimSpace(row.Login))
		e.SubscriberCodes = appendUniq(e.SubscriberCodes, code)
		ownerName(code, row.Subscriber)
		e.addSource(sourceTraffic)
		e.Ours = true
	}

	// Выгрузка биллинга ЭПД видит клиентов ЭПД, которых нет в отчётах API.
	for _, row := range imported {
		id := strings.TrimSpace(row.EDOID)
		e, ok := x.byEDO[id]
		if !ok || id == "" {
			inn, kpp := strings.TrimSpace(row.INN), strings.TrimSpace(row.KPP)
			key := clientKey(inn, kpp)
			if key == "" {
				if id == "" {
					continue
				}
				key = "edo-" + id
			}
			e = entry(key, inn, kpp)
			if id != "" {
				e.EDOIDs = appendUniq(e.EDOIDs, id)
				x.byEDO[id] = e
			}
		}
		e.ClientName = firstNonEmpty(e.ClientName, row.ClientName)
		e.Logins = appendUniq(e.Logins, strings.TrimSpace(row.Login))
		e.SubscriberCodes = appendUniq(e.SubscriberCodes, strings.TrimSpace(row.OwnerCode))
		ownerName(strings.TrimSpace(row.OwnerCode), row.Owner)
		e.addSource(sourceImport)
		e.Ours = true
	}

	for _, sub := range subs {
		for _, org := range sub.Organizations {
			inn, kpp := strings.TrimSpace(org.INN), strings.TrimSpace(org.KPP)
			key := clientKey(inn, kpp)
			if key == "" {
				key = orphanOrgKey(sub.Code, org.Name, inn, kpp)
			}
			e := entry(key, inn, kpp)
			e.ClientName = firstNonEmpty(e.ClientName, org.Name)
			e.SubscriberCodes = appendUniq(e.SubscriberCodes, sub.Code)
			e.InBase = true
			e.addSource(sourceSubscriber)
		}
	}

	for _, row := range requestRows {
		e := x.forRequisites(row.INN, row.KPP)
		if e == nil {
			key := clientKey(row.INN, row.KPP)
			if key == "" {
				continue // заявка без настоящего ИНН ни к кому не относится
			}
			e = entry(key, strings.TrimSpace(row.INN), strings.TrimSpace(row.KPP))
		}
		e.ClientName = firstNonEmpty(e.ClientName, row.CompanyName)
		e.addSource(sourceRequest)
	}

	sort.SliceStable(x.list, func(i, j int) bool {
		return strings.ToLower(x.list[i].ClientName) < strings.ToLower(x.list[j].ClientName)
	})
	for _, e := range x.list {
		for _, code := range e.SubscriberCodes {
			x.bySubscriber[code] = append(x.bySubscriber[code], e)
		}
	}
	return x
}

// forRequisites находит клиента по ИНН и КПП. КПП не указан (так бывает в
// заявке, введённой руками), а у ИНН известен ровно один клиент — это он:
// один ИНН — одно юрлицо. Несколько КПП — не гадаем.
func (x *clientIndex) forRequisites(inn, kpp string) *clientEntry {
	key := clientKey(inn, kpp)
	if key == "" {
		return nil
	}
	if e, ok := x.byKey[key]; ok {
		return e
	}
	inn = strings.TrimSpace(inn)
	if strings.TrimSpace(kpp) == "" && len(x.byINN[inn]) == 1 {
		return x.byINN[inn][0]
	}
	return nil
}

// forRow находит клиента строки отчёта: по идентификатору ЭДО, иначе по реквизитам.
func (x *clientIndex) forRow(edoID, inn, kpp string) *clientEntry {
	if e, ok := x.byEDO[edoID]; ok && edoID != "" {
		return e
	}
	return x.forRequisites(inn, kpp)
}

// resolve находит карточку по ключу. Голый ИНН открывает карточку, если у ИНН
// один клиент: так работают ссылки вида /clients/<ИНН>.
func (x *clientIndex) resolve(key string) *clientEntry {
	if e, ok := x.byKey[key]; ok {
		return e
	}
	if validINN(key) && len(x.byINN[key]) == 1 {
		return x.byINN[key][0]
	}
	return nil
}

// refsOf — клиенты абонента ссылками; пустой список, а не null.
func (x *clientIndex) refsOf(code string) []clientRef {
	list := []clientRef{}
	for _, e := range x.bySubscriber[code] {
		list = append(list, e.clientRef)
	}
	return list
}

func appendUniq(list []string, value string) []string {
	if value == "" || slices.Contains(list, value) {
		return list
	}
	return append(list, value)
}

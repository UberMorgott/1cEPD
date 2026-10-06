# Заявки 1С:ИТС — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Форма, где сотрудник собирает заявку на регистрацию тарифа 1С-ЭПД, сервис проверяет её по правилам «1С» и выдаёт готовый файл `ip00000.xls` для отправки на `itsrobot@1c.ru`.

**Architecture:** Черновики живут в SQLite. Валидация целиком на сервере — она же источник правды для формы. Файл собирается внешней утилитой `xlsfill` (Go не умеет писать BIFF8), которая правит копию шаблона.

**Tech Stack:** Go 1.26, SQLite, Vue 3 + PrimeVue, утилита `tools/xlsfill`.

**Правила:** `docs/REQUESTS.md` — там же таблицы кодов и цены.

---

## Что уже готово

- `tools/xlsfill/xlsfill.py` и собранный `xlsfill.exe`: принимает JSON `{"cells":[{sheet,row,col,value,type}]}`, заполняет `assets/ip00000_ru.xls`, сохраняет BIFF8. Индексы с нуля, по умолчанию значения пишутся текстом.
- `assets/ip00000_ru.xls` — шаблон формы вер. 3.09.
- `internal/partner` — клиент API, `ParseOwner`, `IsFreshOwner`.
- `internal/store` — `Open`, миграции по числу файлов, реестр идентификаторов.
- `internal/httpapi` — `Auth.RequireSession`, `writeJSON`, `writeError`, роутер с четырьмя параметрами.

## Раскладка колонок шаблона (Лист1, индексы с нуля)

Шапка: код партнёра `(1,1)`, пароль `(1,7)`, ответственный `(2,1)`, новый пароль `(2,7)`,
e-mail `(3,1)`, сводный отчёт `(2,10)`, детальный отчёт `(3,10)`.

Таблица регистрации начинается со строки 10. Колонки:
`0` № п/п, `1` код партнёра, `2` способ получения, `3` код дистрибутора, `4` вид 1С:ИТС,
`5` регномер, `6` наименование фирмы, `7` ИНН, `8` КПП, `9` рабочие места, `10` тип деятельности,
`11` директор, `12` ответственный, `13` индекс, `14` город, `15` улица, `16` дом, `17` корпус,
`18` квартира, `19` код города, `20` телефон, `21` факс, `22` e-mail, `23` операция,
`24` дата отказа, `25` причина отказа, `26` дата начала, `27` количество выпусков,
`28` способ оплаты, `29`–`32` регномера 1–4, `33` регномер ПП в Upgrade.

---

## Структура файлов

| Файл | Ответственность |
|---|---|
| `internal/store/migrations/003_requests.sql` | таблица черновиков |
| `internal/store/requests.go` | чтение и запись черновиков |
| `internal/itsreq/tariffs.go` | справочник тарифов ЭПД |
| `internal/itsreq/request.go` | модель заявки |
| `internal/itsreq/validate.go` | правила «1С» |
| `internal/itsreq/cells.go` | раскладка по ячейкам шаблона |
| `internal/itsreq/build.go` | вызов `xlsfill` |
| `internal/httpapi/requests.go` | HTTP-обработчики |
| `web/src/views/RequestsView.vue` | список заявок |
| `web/src/views/RequestEditView.vue` | форма |

---

## Task 1: Справочник тарифов ЭПД

**Files:**
- Create: `internal/itsreq/tariffs.go`
- Test: `internal/itsreq/tariffs_test.go`

- [ ] **Step 1: Написать падающий тест**

```go
package itsreq

import "testing"

func TestTariffsCoverAllThirteenCodes(t *testing.T) {
	if len(Tariffs) != 13 {
		t.Fatalf("тарифов %d, ожидали 13", len(Tariffs))
	}
	for _, code := range []string{
		"2092", "2080", "2081", "2093", "2094", "2095", "2082",
		"2096", "2083", "2840", "2841", "2084", "2085",
	} {
		if _, ok := TariffByCode(code); !ok {
			t.Errorf("код %s отсутствует в справочнике", code)
		}
	}
}

func TestTariffByCodeRejectsUnknown(t *testing.T) {
	// Коды из Менеджера сервиса (Фреш) в файле-заявке недопустимы.
	for _, code := range []string{"1223", "1200", "9999", ""} {
		if _, ok := TariffByCode(code); ok {
			t.Errorf("код %s не должен приниматься", code)
		}
	}
}

func TestTariffCarriesVolumeAndPrices(t *testing.T) {
	tariff, ok := TariffByCode("2083")
	if !ok {
		t.Fatal("код 2083 не найден")
	}
	if tariff.Name != "1С-ЭДО. ЭПД-10000" {
		t.Errorf("название %q", tariff.Name)
	}
	if tariff.Volume != 10000 {
		t.Errorf("объём %d, ожидали 10000", tariff.Volume)
	}
	if tariff.Nomenclature != "2900002632774" {
		t.Errorf("номенклатура %q", tariff.Nomenclature)
	}
	if tariff.RetailKopeks != 4000000 {
		t.Errorf("рекомендованная розница %d, ожидали 4000000", tariff.RetailKopeks)
	}
	if tariff.PartnerKopeks != 2400000 {
		t.Errorf("цена партнёра %d, ожидали 2400000", tariff.PartnerKopeks)
	}
}

func TestFreshCodesAreListedSeparately(t *testing.T) {
	// Для облака Фреш тариф оформляется подпиской, а не заявкой.
	// Справочник нужен, чтобы подсказать сотруднику правильный код.
	code, ok := FreshCodeForVolume(10000)
	if !ok || code != "1203" {
		t.Errorf("код Фреш для 10000 = %q, ожидали 1203", code)
	}
}
```

- [ ] **Step 2: Запустить, убедиться что падает**

Run: `go test ./internal/itsreq/ -v`
Expected: FAIL, `undefined: Tariffs`

- [ ] **Step 3: Реализовать**

```go
// Package itsreq собирает файл-заявку на регистрацию тарифов 1С-ЭПД.
//
// Правила и таблицы кодов взяты из инфовыпуска «1С» № 34677 от 17.07.2026
// и «Справочника партнёра по ИТС», см. docs/REQUESTS.md.
package itsreq

// Tariff — тариф 1С-ЭПД, доступный к оформлению файлом-заявкой.
type Tariff struct {
	// Code идёт в колонку «Вид 1С:ИТС».
	Code string
	Name string
	// Volume — число документов в пакете за год.
	Volume int
	// Nomenclature — номенклатурный номер, нужен для счёта.
	Nomenclature string
	// RetailKopeks и PartnerKopeks хранятся в копейках: цены целые, дробей не бывает.
	RetailKopeks  int64
	PartnerKopeks int64
	// FreshCode — код того же объёма в «Менеджере сервиса» для облака Фреш.
	// В файле-заявке он недопустим, но пригодится в подсказке.
	FreshCode string
}

// Tariffs перечислены по возрастанию объёма.
var Tariffs = []Tariff{
	{"2092", "1С-ЭДО. ЭПД-200", 200, "2900004041864", 140000, 84000, "1223"},
	{"2080", "1С-ЭДО. ЭПД-600", 600, "2900002632743", 360000, 216000, "1200"},
	{"2081", "1С-ЭДО. ЭПД-1000", 1000, "2900002632750", 500000, 300000, "1201"},
	{"2093", "1С-ЭДО. ЭПД-2000", 2000, "2900004041871", 950000, 570000, "1224"},
	{"2094", "1С-ЭДО. ЭПД-3000", 3000, "2900004041888", 1400000, 840000, "1225"},
	{"2095", "1С-ЭДО. ЭПД-4000", 4000, "2900004041895", 1850000, 1110000, "1226"},
	{"2082", "1С-ЭДО. ЭПД-5000", 5000, "2900002632767", 2250000, 1350000, "1202"},
	{"2096", "1С-ЭДО. ЭПД-7000", 7000, "2900004041901", 2950000, 1770000, "1227"},
	{"2083", "1С-ЭДО. ЭПД-10000", 10000, "2900002632774", 4000000, 2400000, "1203"},
	{"2840", "1С-ЭДО. ЭПД-20000", 20000, "2900004041918", 7000000, 4200000, "1228"},
	{"2841", "1С-ЭДО. ЭПД-30000", 30000, "2900004041925", 9600000, 5760000, "1229"},
	{"2084", "1С-ЭДО. ЭПД-50000", 50000, "2900002632781", 15000000, 9000000, "1204"},
	{"2085", "1С-ЭДО. ЭПД-100000", 100000, "2900002632798", 25000000, 15000000, "1205"},
}

// TariffByCode ищет тариф по коду вида 1С:ИТС.
func TariffByCode(code string) (Tariff, bool) {
	for _, tariff := range Tariffs {
		if tariff.Code == code {
			return tariff, true
		}
	}
	return Tariff{}, false
}

// FreshCodeForVolume возвращает код для «Менеджера сервиса» по объёму пакета.
func FreshCodeForVolume(volume int) (string, bool) {
	for _, tariff := range Tariffs {
		if tariff.Volume == volume {
			return tariff.FreshCode, true
		}
	}
	return "", false
}
```

- [ ] **Step 4: Запустить, убедиться что проходит**

Run: `go test ./internal/itsreq/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/itsreq/
git commit -m "feat(itsreq): add EPD tariff reference with codes and prices"
```

---

## Task 2: Модель заявки и правила

**Files:**
- Create: `internal/itsreq/request.go`, `internal/itsreq/validate.go`
- Test: `internal/itsreq/validate_test.go`

- [ ] **Step 1: Написать падающий тест**

```go
package itsreq

import (
	"strings"
	"testing"
)

func validRow() Row {
	return Row{
		TariffCode:   "2083",
		RegNumber:    "18117482",
		CompanyName:  `ООО "Тест"`,
		INN:          "7811000310",
		KPP:          "780001001",
		Workplaces:   5,
		Responsible:  "Иванов Иван Иванович",
		PhoneCode:    "812",
		Phone:        "1234567",
		StartDate:    "01.10.26",
		OwnerCode:    "CL-1000530",
		DeliveryType: "0",
	}
}

func validRequest() Request {
	return Request{
		PartnerCode: "00000",
		Responsible: "Петров Пётр",
		Email:       "zakaz@example.ru",
		Rows:        []Row{validRow()},
	}
}

func TestValidRequestPasses(t *testing.T) {
	if issues := Validate(validRequest()); len(issues) != 0 {
		t.Errorf("ожидали отсутствие замечаний, получили: %+v", issues)
	}
}

func TestFreshOwnerIsRejected(t *testing.T) {
	// Заявка на клиента из облака Фреш уйдёт в брак: там тариф оформляется
	// подпиской в «Менеджере сервиса» другим кодом.
	request := validRequest()
	request.Rows[0].OwnerCode = "FR-FR-600361"

	issues := Validate(request)

	if len(issues) == 0 {
		t.Fatal("владелец FR- обязан блокировать заявку")
	}
	found := false
	for _, issue := range issues {
		if issue.Field == "ownerCode" && issue.Blocking {
			found = true
			if !strings.Contains(issue.Message, "1203") {
				t.Errorf("сообщение должно подсказывать код Фреш: %q", issue.Message)
			}
		}
	}
	if !found {
		t.Errorf("нет блокирующего замечания по ownerCode: %+v", issues)
	}
}

func TestRequiredFields(t *testing.T) {
	cases := map[string]func(*Row){
		"companyName": func(r *Row) { r.CompanyName = "" },
		"inn":         func(r *Row) { r.INN = "" },
		"kpp":         func(r *Row) { r.KPP = "" },
		"regNumber":   func(r *Row) { r.RegNumber = "" },
		"responsible": func(r *Row) { r.Responsible = "" },
		"phone":       func(r *Row) { r.Phone = "" },
		"startDate":   func(r *Row) { r.StartDate = "" },
	}
	for field, damage := range cases {
		request := validRequest()
		damage(&request.Rows[0])

		if !hasIssue(Validate(request), field) {
			t.Errorf("пустое поле %s должно давать замечание", field)
		}
	}
}

func TestTariffCodeMustBeFromList(t *testing.T) {
	request := validRequest()
	request.Rows[0].TariffCode = "1203" // код Фреш, в заявке недопустим

	if !hasIssue(Validate(request), "tariffCode") {
		t.Error("код не из списка ЭПД должен блокировать заявку")
	}
}

func TestStartDateFormat(t *testing.T) {
	// Для сервисов дата в формате ДД.ММ.ГГ, не ММ.ГГ.
	for _, bad := range []string{"10.26", "2026-10-01", "1.10.26", "01.10.2026"} {
		request := validRequest()
		request.Rows[0].StartDate = bad

		if !hasIssue(Validate(request), "startDate") {
			t.Errorf("дата %q должна быть отвергнута", bad)
		}
	}
}

func TestINNAndKPPLength(t *testing.T) {
	request := validRequest()
	request.Rows[0].INN = "78112"
	if !hasIssue(Validate(request), "inn") {
		t.Error("ИНН неверной длины должен давать замечание")
	}

	request = validRequest()
	request.Rows[0].KPP = "7811"
	if !hasIssue(Validate(request), "kpp") {
		t.Error("КПП должен быть девятизначным")
	}
}

func TestDistributorCodeRequiredWhenDelivered(t *testing.T) {
	request := validRequest()
	request.Rows[0].DeliveryType = "1"
	request.Rows[0].DistributorCode = ""

	if !hasIssue(Validate(request), "distributorCode") {
		t.Error("при получении через дистрибьютора его код обязателен")
	}
}

func TestHeaderFieldsRequired(t *testing.T) {
	request := validRequest()
	request.Email = ""
	if !hasIssue(Validate(request), "email") {
		t.Error("e-mail в шапке обязателен: на него приходит протокол")
	}

	request = validRequest()
	request.PartnerCode = "659"
	if !hasIssue(Validate(request), "partnerCode") {
		t.Error("код партнёра должен состоять из пяти цифр")
	}
}

func TestEmptyRequestIsRejected(t *testing.T) {
	request := validRequest()
	request.Rows = nil

	if !hasIssue(Validate(request), "rows") {
		t.Error("заявка без строк не имеет смысла")
	}
}

func hasIssue(issues []Issue, field string) bool {
	for _, issue := range issues {
		if issue.Field == field {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Запустить, убедиться что падает**

Run: `go test ./internal/itsreq/ -run TestValid -v`
Expected: FAIL, `undefined: Row`

- [ ] **Step 3: Создать `internal/itsreq/request.go`**

```go
package itsreq

// Значения, которые правила ЭПД задают жёстко.
const (
	// IssuesCount — «Количество выпусков» для тарифов ЭПД всегда 12 месяцев.
	IssuesCount = "12"
	// PaymentPrepaid — «Способ оплаты»: только предоплата за весь срок.
	PaymentPrepaid = "1"
	// OperationNew — «Операция»: новая регистрация. Отказы для ЭПД не допускаются.
	OperationNew = "0"
)

// Row — строка таблицы регистрации.
type Row struct {
	TariffCode      string `json:"tariffCode"`
	RegNumber       string `json:"regNumber"`
	CompanyName     string `json:"companyName"`
	INN             string `json:"inn"`
	KPP             string `json:"kpp"`
	Workplaces      int    `json:"workplaces"`
	ActivityType    string `json:"activityType"`
	Director        string `json:"director"`
	Responsible     string `json:"responsible"`
	PostalCode      string `json:"postalCode"`
	City            string `json:"city"`
	Street          string `json:"street"`
	House           string `json:"house"`
	Building        string `json:"building"`
	Flat            string `json:"flat"`
	PhoneCode       string `json:"phoneCode"`
	Phone           string `json:"phone"`
	Fax             string `json:"fax"`
	Email           string `json:"email"`
	StartDate       string `json:"startDate"`
	DeliveryType    string `json:"deliveryType"`
	DistributorCode string `json:"distributorCode"`
	// OwnerCode — код абонента-владельца идентификатора ЭДО.
	// В файл не попадает, но решает, можно ли вообще подавать заявку.
	OwnerCode string `json:"ownerCode"`
}

// Request — заявка целиком.
type Request struct {
	ID          int64  `json:"id"`
	PartnerCode string `json:"partnerCode"`
	Responsible string `json:"responsible"`
	Email       string `json:"email"`
	Rows        []Row  `json:"rows"`
}

// Issue — замечание валидатора.
type Issue struct {
	// Row — номер строки таблицы с единицы; ноль означает шапку.
	Row     int    `json:"row"`
	Field   string `json:"field"`
	Message string `json:"message"`
	// Blocking означает, что заявку выпускать нельзя.
	Blocking bool `json:"blocking"`
}
```

- [ ] **Step 4: Создать `internal/itsreq/validate.go`**

```go
package itsreq

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	fiveDigits  = regexp.MustCompile(`^\d{5}$`)
	nineDigits  = regexp.MustCompile(`^\d{9}$`)
	innPattern  = regexp.MustCompile(`^\d{10}$|^\d{12}$`)
	datePattern = regexp.MustCompile(`^\d{2}\.\d{2}\.\d{2}$`)
	emailLike   = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
)

// Validate проверяет заявку по правилам «1С» для тарифов ЭПД.
// Возвращает все найденные замечания, а не первое: заполняющему удобнее
// увидеть список целиком.
func Validate(request Request) []Issue {
	var issues []Issue

	if !fiveDigits.MatchString(request.PartnerCode) {
		issues = append(issues, Issue{
			Field: "partnerCode", Blocking: true,
			Message: "Код партнёра — ровно пять цифр.",
		})
	}
	if !emailLike.MatchString(request.Email) {
		issues = append(issues, Issue{
			Field: "email", Blocking: true,
			Message: "Нужен e-mail: на него придут протокол обработки и счёт.",
		})
	}
	if len(request.Rows) == 0 {
		issues = append(issues, Issue{
			Field: "rows", Blocking: true,
			Message: "В заявке нет ни одной строки.",
		})
	}

	for index, row := range request.Rows {
		issues = append(issues, validateRow(row, index+1)...)
	}
	return issues
}

func validateRow(row Row, number int) []Issue {
	var issues []Issue
	add := func(field, message string, blocking bool) {
		issues = append(issues, Issue{Row: number, Field: field, Message: message, Blocking: blocking})
	}

	tariff, known := TariffByCode(row.TariffCode)
	if !known {
		add("tariffCode",
			"Вид 1С:ИТС должен быть одним из 13 кодов ЭПД, доступных для файла-заявки.", true)
	}

	// Самая дорогая ошибка в теме: для облака Фреш заявка не работает.
	if strings.HasPrefix(row.OwnerCode, "FR-") {
		hint := "оформите подписку в «Менеджере сервиса»"
		if known {
			hint = fmt.Sprintf("оформите подписку в «Менеджере сервиса», код %s", tariff.FreshCode)
		}
		add("ownerCode", fmt.Sprintf(
			"Клиент работает в облаке Фреш (владелец %s). Файл-заявка на него не действует: %s.",
			row.OwnerCode, hint), true)
	}

	if strings.TrimSpace(row.CompanyName) == "" {
		add("companyName", "Наименование фирмы обязательно.", true)
	}
	if !innPattern.MatchString(row.INN) {
		add("inn", "ИНН обязателен: 10 цифр у организации, 12 у ИП.", true)
	}
	if !nineDigits.MatchString(row.KPP) {
		add("kpp", "КПП обязателен для тарифов ЭПД и состоит из девяти цифр.", true)
	}
	if strings.TrimSpace(row.RegNumber) == "" {
		add("regNumber", "Регистрационный номер программы обязателен.", true)
	}
	if strings.TrimSpace(row.Responsible) == "" {
		add("responsible", "Укажите ответственного за программный продукт.", true)
	}
	if strings.TrimSpace(row.Phone) == "" {
		add("phone", "Телефон обязателен.", true)
	}
	if row.Workplaces <= 0 {
		add("workplaces", "Количество рабочих мест должно быть больше нуля.", true)
	}

	// Для сервисов дата идёт с днём, в отличие от обычного ИТС.
	if !datePattern.MatchString(row.StartDate) {
		add("startDate", "Дата начала указывается в формате ДД.ММ.ГГ, например 01.10.26.", true)
	}

	switch row.DeliveryType {
	case "0":
		// Самовывоз: код дистрибьютора не нужен.
	case "1":
		if !fiveDigits.MatchString(row.DistributorCode) {
			add("distributorCode",
				"При получении через дистрибьютора укажите его код из пяти цифр.", true)
		}
	default:
		add("deliveryType", "Способ получения: 0 — самовывоз, 1 — через дистрибьютора.", true)
	}

	return issues
}

// Blocking сообщает, есть ли среди замечаний хотя бы одно, запрещающее выпуск файла.
func Blocking(issues []Issue) bool {
	for _, issue := range issues {
		if issue.Blocking {
			return true
		}
	}
	return false
}
```

- [ ] **Step 5: Запустить, убедиться что проходит**

Run: `go test ./internal/itsreq/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/itsreq/
git commit -m "feat(itsreq): validate requests against the 1C rules for EPD"
```

---

## Task 3: Раскладка по ячейкам

**Files:**
- Create: `internal/itsreq/cells.go`
- Test: `internal/itsreq/cells_test.go`

- [ ] **Step 1: Написать падающий тест**

```go
package itsreq

import "testing"

func cellAt(cells []Cell, row, col int) (Cell, bool) {
	for _, cell := range cells {
		if cell.Row == row && cell.Col == col {
			return cell, true
		}
	}
	return Cell{}, false
}

func TestCellsFillHeader(t *testing.T) {
	cells := Cells(validRequest())

	partner, ok := cellAt(cells, 1, 1)
	if !ok || partner.Value != "00000" {
		t.Errorf("код партнёра не на месте: %+v", partner)
	}
	email, ok := cellAt(cells, 3, 1)
	if !ok || email.Value != "zakaz@example.ru" {
		t.Errorf("e-mail не на месте: %+v", email)
	}
}

func TestCellsFillFirstRowAtIndexTen(t *testing.T) {
	cells := Cells(validRequest())

	number, ok := cellAt(cells, 10, 0)
	if !ok || number.Value != "1" {
		t.Errorf("номер строки: %+v", number)
	}
	tariff, ok := cellAt(cells, 10, 4)
	if !ok || tariff.Value != "2083" {
		t.Errorf("вид 1С:ИТС: %+v", tariff)
	}
	inn, ok := cellAt(cells, 10, 7)
	if !ok || inn.Value != "7811000310" {
		t.Errorf("ИНН: %+v", inn)
	}
}

func TestCellsForceFixedValues(t *testing.T) {
	// Количество выпусков и способ оплаты для ЭПД заданы правилами,
	// вводить их руками нельзя.
	cells := Cells(validRequest())

	issues, ok := cellAt(cells, 10, 27)
	if !ok || issues.Value != "12" {
		t.Errorf("количество выпусков должно быть 12, получили %+v", issues)
	}
	payment, ok := cellAt(cells, 10, 28)
	if !ok || payment.Value != "1" {
		t.Errorf("способ оплаты должен быть 1, получили %+v", payment)
	}
	operation, ok := cellAt(cells, 10, 23)
	if !ok || operation.Value != "0" {
		t.Errorf("операция должна быть 0, получили %+v", operation)
	}
}

func TestCellsKeepCodesAsText(t *testing.T) {
	// «0» и «1» обязаны остаться строками: числом робот прочитает их иначе.
	cells := Cells(validRequest())

	for _, col := range []int{2, 23, 28} {
		cell, ok := cellAt(cells, 10, col)
		if !ok {
			t.Fatalf("колонка %d отсутствует", col)
		}
		if cell.Type == "number" {
			t.Errorf("колонка %d должна писаться текстом", col)
		}
	}
}

func TestCellsNumberSecondRow(t *testing.T) {
	request := validRequest()
	request.Rows = append(request.Rows, validRow())

	cells := Cells(request)

	second, ok := cellAt(cells, 11, 0)
	if !ok || second.Value != "2" {
		t.Errorf("вторая строка должна получить номер 2: %+v", second)
	}
}

func TestCellsOmitEmptyOptionalFields(t *testing.T) {
	cells := Cells(validRequest())

	// Факс не заполнен — пустую ячейку писать незачем.
	if _, ok := cellAt(cells, 10, 21); ok {
		t.Error("пустой факс не должен попадать в файл")
	}
}
```

- [ ] **Step 2: Запустить, убедиться что падает**

Run: `go test ./internal/itsreq/ -run TestCells -v`
Expected: FAIL, `undefined: Cells`

- [ ] **Step 3: Реализовать**

```go
package itsreq

import "strconv"

// firstDataRow — строка шаблона, с которой начинается таблица регистрации.
// Нумерация с нуля, как в самом файле.
const firstDataRow = 10

// Cell — одна ячейка для утилиты xlsfill.
type Cell struct {
	Sheet int    `json:"sheet"`
	Row   int    `json:"row"`
	Col   int    `json:"col"`
	Value string `json:"value"`
	// Type пуст для текста; "number" ставится только там, где робот ждёт число.
	Type string `json:"type,omitempty"`
}

// Cells раскладывает заявку по ячейкам шаблона.
//
// Всё пишется текстом намеренно: коды «0» и «1», даты «01.10.26» и номера
// с ведущими нулями, став числами, будут прочитаны роботом иначе.
// Пустые необязательные поля пропускаются — шаблон уже содержит нужное оформление.
func Cells(request Request) []Cell {
	cells := make([]Cell, 0, 32)

	put := func(row, col int, value string) {
		if value == "" {
			return
		}
		cells = append(cells, Cell{Row: row, Col: col, Value: value})
	}

	// Шапка «Информация об отправителе заявки».
	put(1, 1, request.PartnerCode)
	put(2, 1, request.Responsible)
	put(3, 1, request.Email)

	for index, row := range request.Rows {
		line := firstDataRow + index

		put(line, 0, strconv.Itoa(index+1))
		put(line, 1, request.PartnerCode)
		put(line, 2, row.DeliveryType)
		put(line, 3, row.DistributorCode)
		put(line, 4, row.TariffCode)
		put(line, 5, row.RegNumber)
		put(line, 6, row.CompanyName)
		put(line, 7, row.INN)
		put(line, 8, row.KPP)
		if row.Workplaces > 0 {
			put(line, 9, strconv.Itoa(row.Workplaces))
		}
		put(line, 10, row.ActivityType)
		put(line, 11, row.Director)
		put(line, 12, row.Responsible)
		put(line, 13, row.PostalCode)
		put(line, 14, row.City)
		put(line, 15, row.Street)
		put(line, 16, row.House)
		put(line, 17, row.Building)
		put(line, 18, row.Flat)
		put(line, 19, row.PhoneCode)
		put(line, 20, row.Phone)
		put(line, 21, row.Fax)
		put(line, 22, row.Email)
		put(line, 23, OperationNew)
		put(line, 26, row.StartDate)
		put(line, 27, IssuesCount)
		put(line, 28, PaymentPrepaid)
	}

	return cells
}
```

- [ ] **Step 4: Запустить, убедиться что проходит**

Run: `go test ./internal/itsreq/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/itsreq/
git commit -m "feat(itsreq): map a request onto the template cells"
```

---

## Task 4: Сборка файла через xlsfill

**Files:**
- Create: `internal/itsreq/build.go`
- Test: `internal/itsreq/build_test.go`

- [ ] **Step 1: Написать падающий тест**

```go
package itsreq

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeFiller изображает утилиту: читает JSON со stdin и создаёт файл.
func fakeFiller(t *testing.T) string {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("заглушка написана под cmd")
	}

	path := filepath.Join(t.TempDir(), "fake.cmd")
	script := "@echo off\r\nmore > \"%~4\"\r\necho {\"ok\":true}\r\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatalf("не создать заглушку: %v", err)
	}
	return path
}

func TestBuildRefusesInvalidRequest(t *testing.T) {
	builder := NewBuilder("нет.exe", "нет.xls")
	request := validRequest()
	request.Rows[0].OwnerCode = "FR-FR-1"

	_, err := builder.Build(context.Background(), request, t.TempDir())

	if err == nil {
		t.Fatal("заявка с блокирующим замечанием не должна собираться")
	}
	if !strings.Contains(err.Error(), "Фреш") {
		t.Errorf("ошибка должна называть причину: %v", err)
	}
}

func TestBuildPassesCellsToFiller(t *testing.T) {
	// Проверяем, что утилите уходит корректный JSON с ячейками.
	payloadPath := filepath.Join(t.TempDir(), "payload.json")
	builder := NewBuilder(fakeFiller(t), "template.xls")
	builder.payloadPathForTest = payloadPath

	out, err := builder.Build(context.Background(), validRequest(), t.TempDir())
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if !strings.HasSuffix(out, ".xls") {
		t.Errorf("результат должен быть .xls, получили %q", out)
	}

	raw, err := os.ReadFile(payloadPath)
	if err != nil {
		t.Fatalf("payload не записан: %v", err)
	}
	var payload struct {
		Cells []Cell `json:"cells"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("payload не разбирается: %v", err)
	}
	if len(payload.Cells) == 0 {
		t.Error("в payload нет ячеек")
	}
}

func TestFileNameUsesPartnerCode(t *testing.T) {
	if got := FileName("00000"); got != "ip00000.xls" {
		t.Errorf("имя файла = %q, ожидали ip00000.xls", got)
	}
}
```

- [ ] **Step 2: Запустить, убедиться что падает**

Run: `go test ./internal/itsreq/ -run TestBuild -v`
Expected: FAIL, `undefined: NewBuilder`

- [ ] **Step 3: Реализовать**

```go
package itsreq

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// fillTimeout ограничивает работу внешней утилиты: она заполняет один файл,
// секунды здесь с запасом.
const fillTimeout = 30 * time.Second

// Builder собирает файл-заявку, вызывая утилиту xlsfill.
//
// Go не умеет писать BIFF8, а робот принимает только его, поэтому запись
// вынесена во внешний процесс. Подробности в tools/xlsfill/README.md.
type Builder struct {
	fillerPath   string
	templatePath string

	// payloadPathForTest позволяет тесту заглянуть в переданный JSON.
	payloadPathForTest string
}

// NewBuilder создаёт сборщик.
func NewBuilder(fillerPath, templatePath string) *Builder {
	return &Builder{fillerPath: fillerPath, templatePath: templatePath}
}

// FileName возвращает имя файла по правилам «1С»: ip плюс код партнёра.
func FileName(partnerCode string) string {
	return "ip" + partnerCode + ".xls"
}

// Build проверяет заявку, раскладывает её по ячейкам и вызывает утилиту.
// Возвращает путь к готовому файлу внутри outDir.
func (b *Builder) Build(ctx context.Context, request Request, outDir string) (string, error) {
	issues := Validate(request)
	if Blocking(issues) {
		return "", fmt.Errorf("itsreq: заявка не прошла проверку: %s", firstBlocking(issues))
	}

	payload, err := json.Marshal(map[string]any{"cells": Cells(request)})
	if err != nil {
		return "", fmt.Errorf("itsreq: не собрать данные для заполнения: %w", err)
	}

	payloadPath := b.payloadPathForTest
	if payloadPath == "" {
		payloadPath = filepath.Join(outDir, "payload.json")
	}
	if err := os.WriteFile(payloadPath, payload, 0o600); err != nil {
		return "", fmt.Errorf("itsreq: не записать данные для заполнения: %w", err)
	}
	defer os.Remove(payloadPath)

	outPath := filepath.Join(outDir, FileName(request.PartnerCode))

	ctx, cancel := context.WithTimeout(ctx, fillTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, b.fillerPath,
		"--template", b.templatePath,
		"--out", outPath,
		"--data", payloadPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("itsreq: утилита заполнения не отработала: %w: %s",
			err, strings.TrimSpace(string(output)))
	}

	if _, err := os.Stat(outPath); err != nil {
		return "", fmt.Errorf("itsreq: файл не создан: %w", err)
	}
	return outPath, nil
}

func firstBlocking(issues []Issue) string {
	for _, issue := range issues {
		if issue.Blocking {
			if issue.Row > 0 {
				return fmt.Sprintf("строка %d: %s", issue.Row, issue.Message)
			}
			return issue.Message
		}
	}
	return "неизвестная причина"
}
```

- [ ] **Step 4: Запустить, убедиться что проходит**

Run: `go test ./internal/itsreq/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/itsreq/
git commit -m "feat(itsreq): build the request file through the xlsfill utility"
```

---

## Task 5: Хранение черновиков

**Files:**
- Create: `internal/store/migrations/003_requests.sql`, `internal/store/requests.go`
- Test: `internal/store/requests_test.go`

- [ ] **Step 1: Написать падающий тест**

```go
package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func testRequests(t *testing.T) *Requests {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewRequests(db)
}

func TestSaveAndLoadDraft(t *testing.T) {
	s := testRequests(t)
	ctx := context.Background()

	id, err := s.Save(ctx, RequestDraft{
		Title:      "ЭПД-10000 для Тест",
		Status:     "draft",
		SchemaVer:  "3.09",
		PayloadJSON: `{"partnerCode":"00000"}`,
	}, time.Now())
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	draft, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if draft.Title != "ЭПД-10000 для Тест" {
		t.Errorf("название %q", draft.Title)
	}
	if draft.Revision != 1 {
		t.Errorf("ревизия %d, ожидали 1", draft.Revision)
	}
}

func TestUpdateBumpsRevision(t *testing.T) {
	s := testRequests(t)
	ctx := context.Background()

	id, _ := s.Save(ctx, RequestDraft{Title: "первая", Status: "draft", SchemaVer: "3.09",
		PayloadJSON: "{}"}, time.Now())

	draft, _ := s.Get(ctx, id)
	draft.Title = "вторая"
	if err := s.Update(ctx, draft, time.Now()); err != nil {
		t.Fatalf("Update: %v", err)
	}

	updated, _ := s.Get(ctx, id)
	if updated.Revision != 2 {
		t.Errorf("ревизия %d, ожидали 2", updated.Revision)
	}
	if updated.Title != "вторая" {
		t.Errorf("название не обновилось: %q", updated.Title)
	}
}

func TestUpdateRejectsStaleRevision(t *testing.T) {
	// Двое редактируют один черновик: второй не должен затирать чужие правки молча.
	s := testRequests(t)
	ctx := context.Background()

	id, _ := s.Save(ctx, RequestDraft{Title: "исходная", Status: "draft", SchemaVer: "3.09",
		PayloadJSON: "{}"}, time.Now())

	first, _ := s.Get(ctx, id)
	second, _ := s.Get(ctx, id)

	first.Title = "правка первого"
	if err := s.Update(ctx, first, time.Now()); err != nil {
		t.Fatalf("первое обновление: %v", err)
	}

	second.Title = "правка второго"
	if err := s.Update(ctx, second, time.Now()); err != ErrStaleRevision {
		t.Errorf("err = %v, ожидали ErrStaleRevision", err)
	}
}

func TestListReturnsNewestFirst(t *testing.T) {
	s := testRequests(t)
	ctx := context.Background()
	now := time.Now()

	s.Save(ctx, RequestDraft{Title: "старая", Status: "draft", SchemaVer: "3.09",
		PayloadJSON: "{}"}, now)
	s.Save(ctx, RequestDraft{Title: "новая", Status: "draft", SchemaVer: "3.09",
		PayloadJSON: "{}"}, now.Add(time.Hour))

	list, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 || list[0].Title != "новая" {
		t.Errorf("порядок нарушен: %+v", list)
	}
}

func TestMarkExported(t *testing.T) {
	s := testRequests(t)
	ctx := context.Background()
	id, _ := s.Save(ctx, RequestDraft{Title: "заявка", Status: "draft", SchemaVer: "3.09",
		PayloadJSON: "{}"}, time.Now())

	if err := s.MarkExported(ctx, id, time.Now()); err != nil {
		t.Fatalf("MarkExported: %v", err)
	}

	draft, _ := s.Get(ctx, id)
	if draft.Status != "exported" {
		t.Errorf("статус %q, ожидали exported", draft.Status)
	}
	if draft.ExportedAt == nil {
		t.Error("дата выгрузки не проставлена")
	}
}
```

- [ ] **Step 2: Запустить, убедиться что падает**

Run: `go test ./internal/store/ -run TestSaveAndLoadDraft -v`
Expected: FAIL, `undefined: NewRequests`

- [ ] **Step 3: Создать `internal/store/migrations/003_requests.sql`**

```sql
-- Черновики заявок. Статуса «отправлена» нет намеренно: сервис не отправляет
-- почту и знать об отправке не может, файл уносит человек.
CREATE TABLE its_requests (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    title        TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'draft',
    schema_ver   TEXT NOT NULL DEFAULT '3.09',
    revision     INTEGER NOT NULL DEFAULT 1,
    payload_json TEXT NOT NULL,
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL,
    exported_at  INTEGER
);
CREATE INDEX idx_its_requests_updated ON its_requests(updated_at DESC);
```

- [ ] **Step 4: Создать `internal/store/requests.go`**

```go
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrStaleRevision означает, что черновик успели изменить в другом окне.
var ErrStaleRevision = errors.New("store: черновик изменён другим пользователем")

// RequestDraft — сохранённая заявка.
type RequestDraft struct {
	ID          int64
	Title       string
	Status      string
	SchemaVer   string
	Revision    int64
	PayloadJSON string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ExportedAt  *time.Time
}

// Requests хранит черновики заявок.
type Requests struct {
	db *sql.DB
}

// NewRequests создаёт хранилище поверх открытой базы.
func NewRequests(db *sql.DB) *Requests {
	return &Requests{db: db}
}

// Save создаёт новый черновик.
func (s *Requests) Save(ctx context.Context, draft RequestDraft, at time.Time) (int64, error) {
	unix := at.UTC().Unix()
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO its_requests (title, status, schema_ver, revision, payload_json, created_at, updated_at)
		VALUES (?, ?, ?, 1, ?, ?, ?)`,
		draft.Title, draft.Status, draft.SchemaVer, draft.PayloadJSON, unix, unix)
	if err != nil {
		return 0, fmt.Errorf("store: не сохранить заявку: %w", err)
	}
	return result.LastInsertId()
}

// Update перезаписывает черновик, если его не изменили параллельно.
func (s *Requests) Update(ctx context.Context, draft RequestDraft, at time.Time) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE its_requests
		SET title = ?, status = ?, payload_json = ?, revision = revision + 1, updated_at = ?
		WHERE id = ? AND revision = ?`,
		draft.Title, draft.Status, draft.PayloadJSON, at.UTC().Unix(), draft.ID, draft.Revision)
	if err != nil {
		return fmt.Errorf("store: не обновить заявку: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: не проверить обновление заявки: %w", err)
	}
	if affected == 0 {
		return ErrStaleRevision
	}
	return nil
}

// Get читает черновик.
func (s *Requests) Get(ctx context.Context, id int64) (RequestDraft, error) {
	var draft RequestDraft
	var created, updated int64
	var exported sql.NullInt64

	err := s.db.QueryRowContext(ctx, `
		SELECT id, title, status, schema_ver, revision, payload_json, created_at, updated_at, exported_at
		FROM its_requests WHERE id = ?`, id).
		Scan(&draft.ID, &draft.Title, &draft.Status, &draft.SchemaVer, &draft.Revision,
			&draft.PayloadJSON, &created, &updated, &exported)
	if err != nil {
		return RequestDraft{}, fmt.Errorf("store: не прочитать заявку %d: %w", id, err)
	}

	draft.CreatedAt = time.Unix(created, 0).UTC()
	draft.UpdatedAt = time.Unix(updated, 0).UTC()
	if exported.Valid {
		value := time.Unix(exported.Int64, 0).UTC()
		draft.ExportedAt = &value
	}
	return draft, nil
}

// List возвращает заявки, свежие сверху.
func (s *Requests) List(ctx context.Context) ([]RequestDraft, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, title, status, schema_ver, revision, created_at, updated_at, exported_at
		FROM its_requests ORDER BY updated_at DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("store: не прочитать список заявок: %w", err)
	}
	defer rows.Close()

	var list []RequestDraft
	for rows.Next() {
		var draft RequestDraft
		var created, updated int64
		var exported sql.NullInt64
		if err := rows.Scan(&draft.ID, &draft.Title, &draft.Status, &draft.SchemaVer,
			&draft.Revision, &created, &updated, &exported); err != nil {
			return nil, fmt.Errorf("store: не разобрать заявку: %w", err)
		}
		draft.CreatedAt = time.Unix(created, 0).UTC()
		draft.UpdatedAt = time.Unix(updated, 0).UTC()
		if exported.Valid {
			value := time.Unix(exported.Int64, 0).UTC()
			draft.ExportedAt = &value
		}
		list = append(list, draft)
	}
	return list, rows.Err()
}

// MarkExported отмечает, что по заявке выгружен файл.
func (s *Requests) MarkExported(ctx context.Context, id int64, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE its_requests SET status = 'exported', exported_at = ?, updated_at = ?
		WHERE id = ?`, at.UTC().Unix(), at.UTC().Unix(), id)
	if err != nil {
		return fmt.Errorf("store: не отметить выгрузку заявки %d: %w", id, err)
	}
	return nil
}
```

- [ ] **Step 5: Запустить, убедиться что проходит**

Run: `go test ./internal/store/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/store/
git commit -m "feat(store): keep ITS request drafts with optimistic locking"
```

---

## Task 6: HTTP-обработчики заявок

**Files:**
- Create: `internal/httpapi/requests.go`
- Modify: `internal/httpapi/router.go`, `cmd/server/main.go`, `internal/config/config.go`
- Test: `internal/httpapi/requests_test.go`

- [ ] **Step 1: Написать падающий тест**

```go
package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTariffsEndpointListsThirteen(t *testing.T) {
	h := NewRequests(nil, nil)

	rec := httptest.NewRecorder()
	h.Tariffs(rec, httptest.NewRequest(http.MethodGet, "/api/its/tariffs", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("код %d", rec.Code)
	}
	var body struct {
		Tariffs []struct {
			Code string `json:"code"`
			Name string `json:"name"`
		} `json:"tariffs"`
	}
	json.Unmarshal(rec.Body.Bytes(), &body)
	if len(body.Tariffs) != 13 {
		t.Errorf("тарифов %d, ожидали 13", len(body.Tariffs))
	}
}

func TestValidateEndpointReportsFreshOwner(t *testing.T) {
	h := NewRequests(nil, nil)

	payload := `{"partnerCode":"00000","email":"a@b.ru","responsible":"Пётр","rows":[
		{"tariffCode":"2083","regNumber":"1","companyName":"ООО","inn":"7811000310",
		 "kpp":"780001001","workplaces":1,"responsible":"Иван","phoneCode":"812",
		 "phone":"1","startDate":"01.10.26","deliveryType":"0","ownerCode":"FR-FR-1"}]}`

	req := httptest.NewRequest(http.MethodPost, "/api/its/validate", strings.NewReader(payload))
	rec := httptest.NewRecorder()
	h.Validate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("код %d, проверка должна отвечать 200 со списком замечаний", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Фреш") {
		t.Errorf("в ответе нет замечания про Фреш: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"blocking":true`) {
		t.Error("замечание должно быть блокирующим")
	}
}

func TestValidateEndpointAcceptsGoodRequest(t *testing.T) {
	h := NewRequests(nil, nil)

	payload := `{"partnerCode":"00000","email":"a@b.ru","responsible":"Пётр","rows":[
		{"tariffCode":"2083","regNumber":"1","companyName":"ООО","inn":"7811000310",
		 "kpp":"780001001","workplaces":1,"responsible":"Иван","phoneCode":"812",
		 "phone":"1","startDate":"01.10.26","deliveryType":"0","ownerCode":"CL-1"}]}`

	req := httptest.NewRequest(http.MethodPost, "/api/its/validate", strings.NewReader(payload))
	rec := httptest.NewRecorder()
	h.Validate(rec, req)

	var body struct {
		Issues   []any `json:"issues"`
		Blocking bool  `json:"blocking"`
	}
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Blocking {
		t.Errorf("корректная заявка не должна блокироваться: %s", rec.Body.String())
	}
}
```

- [ ] **Step 2: Запустить, убедиться что падает**

Run: `go test ./internal/httpapi/ -run TestTariffs -v`
Expected: FAIL, `undefined: NewRequests`

- [ ] **Step 3: Реализовать `internal/httpapi/requests.go`**

```go
package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"time"

	"partnerops/internal/itsreq"
	"partnerops/internal/store"
)

// RequestStore — то, что нужно обработчикам от хранилища заявок.
type RequestStore interface {
	Save(ctx context.Context, draft store.RequestDraft, at time.Time) (int64, error)
	Update(ctx context.Context, draft store.RequestDraft, at time.Time) error
	Get(ctx context.Context, id int64) (store.RequestDraft, error)
	List(ctx context.Context) ([]store.RequestDraft, error)
	MarkExported(ctx context.Context, id int64, at time.Time) error
}

// FileBuilder собирает файл-заявку.
type FileBuilder interface {
	Build(ctx context.Context, request itsreq.Request, outDir string) (string, error)
}

// Requests обслуживает экран заявок.
type Requests struct {
	store   RequestStore
	builder FileBuilder
}

// NewRequests создаёт обработчики.
func NewRequests(store RequestStore, builder FileBuilder) *Requests {
	return &Requests{store: store, builder: builder}
}

// Tariffs отдаёт справочник тарифов ЭПД для выпадающего списка.
func (h *Requests) Tariffs(w http.ResponseWriter, r *http.Request) {
	type item struct {
		Code      string `json:"code"`
		Name      string `json:"name"`
		Volume    int    `json:"volume"`
		Retail    string `json:"retail"`
		Partner   string `json:"partner"`
		FreshCode string `json:"freshCode"`
	}

	list := make([]item, 0, len(itsreq.Tariffs))
	for _, tariff := range itsreq.Tariffs {
		list = append(list, item{
			Code: tariff.Code, Name: tariff.Name, Volume: tariff.Volume,
			Retail:  formatKopeks(tariff.RetailKopeks),
			Partner: formatKopeks(tariff.PartnerKopeks),
			FreshCode: tariff.FreshCode,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"tariffs": list})
}

// Validate проверяет заявку и возвращает замечания.
// Отвечает 200 даже при замечаниях: это не ошибка запроса, а результат проверки.
func (h *Requests) Validate(w http.ResponseWriter, r *http.Request) {
	request, ok := decodeRequest(w, r)
	if !ok {
		return
	}

	issues := itsreq.Validate(request)
	writeJSON(w, http.StatusOK, map[string]any{
		"issues":   issues,
		"blocking": itsreq.Blocking(issues),
	})
}

// Download собирает файл и отдаёт его на скачивание.
func (h *Requests) Download(w http.ResponseWriter, r *http.Request) {
	request, ok := decodeRequest(w, r)
	if !ok {
		return
	}

	dir, err := os.MkdirTemp("", "itsreq")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось подготовить файл.")
		return
	}
	defer os.RemoveAll(dir)

	path, err := h.builder.Build(r.Context(), request, dir)
	if err != nil {
		writeError(w, http.StatusBadRequest, "build_failed", err.Error())
		return
	}

	file, err := os.Open(path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Файл собран, но не читается.")
		return
	}
	defer file.Close()

	name := itsreq.FileName(request.PartnerCode)
	w.Header().Set("Content-Type", "application/vnd.ms-excel")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	http.ServeContent(w, r, name, time.Now(), file)
}

// List отдаёт сохранённые заявки.
func (h *Requests) List(w http.ResponseWriter, r *http.Request) {
	drafts, err := h.store.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось прочитать заявки.")
		return
	}

	type item struct {
		ID         int64  `json:"id"`
		Title      string `json:"title"`
		Status     string `json:"status"`
		Revision   int64  `json:"revision"`
		UpdatedAt  string `json:"updatedAt"`
		ExportedAt string `json:"exportedAt,omitempty"`
	}

	list := make([]item, 0, len(drafts))
	for _, draft := range drafts {
		entry := item{
			ID: draft.ID, Title: draft.Title, Status: draft.Status,
			Revision: draft.Revision, UpdatedAt: draft.UpdatedAt.Format(time.RFC3339),
		}
		if draft.ExportedAt != nil {
			entry.ExportedAt = draft.ExportedAt.Format(time.RFC3339)
		}
		list = append(list, entry)
	}
	writeJSON(w, http.StatusOK, map[string]any{"requests": list})
}

// Save создаёт или обновляет черновик.
func (h *Requests) Save(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID       int64           `json:"id"`
		Title    string          `json:"title"`
		Revision int64           `json:"revision"`
		Request  itsreq.Request  `json:"request"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Неверный формат запроса.")
		return
	}

	payload, err := json.Marshal(body.Request)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Не удалось сохранить данные заявки.")
		return
	}

	now := time.Now().UTC()
	draft := store.RequestDraft{
		ID: body.ID, Title: body.Title, Status: "draft", SchemaVer: "3.09",
		Revision: body.Revision, PayloadJSON: string(payload),
	}

	if body.ID == 0 {
		id, err := h.store.Save(r.Context(), draft, now)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "Не удалось сохранить заявку.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": id, "revision": 1})
		return
	}

	if err := h.store.Update(r.Context(), draft, now); err != nil {
		if err == store.ErrStaleRevision {
			writeError(w, http.StatusConflict, "stale",
				"Заявку изменили в другом окне. Обновите страницу.")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "Не удалось сохранить заявку.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": body.ID, "revision": body.Revision + 1})
}

// Get отдаёт один черновик.
func (h *Requests) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Неверный идентификатор заявки.")
		return
	}

	draft, err := h.store.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Заявка не найдена.")
		return
	}

	var request itsreq.Request
	json.Unmarshal([]byte(draft.PayloadJSON), &request)

	writeJSON(w, http.StatusOK, map[string]any{
		"id": draft.ID, "title": draft.Title, "status": draft.Status,
		"revision": draft.Revision, "request": request,
	})
}

func decodeRequest(w http.ResponseWriter, r *http.Request) (itsreq.Request, bool) {
	var request itsreq.Request
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Неверный формат заявки.")
		return itsreq.Request{}, false
	}
	return request, true
}

// formatKopeks выводит цену без дробной части, когда копеек нет.
func formatKopeks(value int64) string {
	rubles := value / 100
	kopeks := value % 100
	if kopeks == 0 {
		return strconv.FormatInt(rubles, 10)
	}
	return strconv.FormatInt(rubles, 10) + "," + strconv.FormatInt(kopeks, 10)
}
```

- [ ] **Step 4: Добавить настройки в `internal/config/config.go`**

В структуру `Config` добавить:

```go
	// XLSFillPath и TemplatePath нужны для сборки файла-заявки.
	XLSFillPath  string
	TemplatePath string
```

В `Load` после остальных значений:

```go
	cfg.XLSFillPath = orDefault(get("XLSFILL_PATH"), "./tools/xlsfill/dist/xlsfill.exe")
	cfg.TemplatePath = orDefault(get("TEMPLATE_PATH"), "./assets/ip00000_ru.xls")
```

- [ ] **Step 5: Подключить маршруты в `internal/httpapi/router.go`**

Добавить пятый параметр `requests *Requests` и маршруты:

```go
	if requests != nil {
		mux.Handle("GET /api/its/tariffs", auth.RequireSession(http.HandlerFunc(requests.Tariffs)))
		mux.Handle("POST /api/its/validate", auth.RequireSession(http.HandlerFunc(requests.Validate)))
		mux.Handle("POST /api/its/download", auth.RequireSession(http.HandlerFunc(requests.Download)))
		mux.Handle("GET /api/its/requests", auth.RequireSession(http.HandlerFunc(requests.List)))
		mux.Handle("POST /api/its/requests", auth.RequireSession(http.HandlerFunc(requests.Save)))
		mux.Handle("GET /api/its/requests/{id}", auth.RequireSession(http.HandlerFunc(requests.Get)))
	}
```

В существующих тестах роутера передавать `nil` пятым аргументом.

- [ ] **Step 6: Подключить в `cmd/server/main.go`**

```go
	requestsAPI := httpapi.NewRequests(
		store.NewRequests(db),
		itsreq.NewBuilder(cfg.XLSFillPath, cfg.TemplatePath),
	)
	router := httpapi.NewRouter(auth, httpapi.NewEvents(bus), registryAPI, frontend, requestsAPI)
```

- [ ] **Step 7: Проверить**

Run: `go build ./... && go test ./... -race`
Expected: всё зелёное

- [ ] **Step 8: Commit**

```bash
git add internal/httpapi/ internal/config/ cmd/server/
git commit -m "feat(httpapi): expose ITS request drafts, validation and download"
```

---

## Task 7: Экран заявок

**Files:**
- Create: `web/src/views/RequestsView.vue`
- Modify: `web/src/api/client.ts`, `web/src/router/index.ts`, `web/src/App.vue`

- [ ] **Step 1: Дополнить `web/src/api/client.ts`**

```ts
export interface ItsTariff {
  code: string
  name: string
  volume: number
  retail: string
  partner: string
  freshCode: string
}

export interface ItsIssue {
  row: number
  field: string
  message: string
  blocking: boolean
}

export interface ItsRow {
  tariffCode: string
  regNumber: string
  companyName: string
  inn: string
  kpp: string
  workplaces: number
  activityType: string
  director: string
  responsible: string
  postalCode: string
  city: string
  street: string
  house: string
  building: string
  flat: string
  phoneCode: string
  phone: string
  fax: string
  email: string
  startDate: string
  deliveryType: string
  distributorCode: string
  ownerCode: string
}

export interface ItsRequest {
  partnerCode: string
  responsible: string
  email: string
  rows: ItsRow[]
}
```

И добавить в объект `api`:

```ts
  itsTariffs: () => request<{ tariffs: ItsTariff[] }>('/api/its/tariffs'),

  itsValidate: (payload: ItsRequest) =>
    request<{ issues: ItsIssue[]; blocking: boolean }>('/api/its/validate', {
      method: 'POST',
      body: JSON.stringify(payload),
    }),
```

- [ ] **Step 2: Создать `web/src/views/RequestsView.vue`**

```vue
<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import InputNumber from 'primevue/inputnumber'
import Select from 'primevue/select'
import Message from 'primevue/message'
import { api, type ItsIssue, type ItsRequest, type ItsRow, type ItsTariff } from '../api/client'

const tariffs = ref<ItsTariff[]>([])
const issues = ref<ItsIssue[]>([])
const blocking = ref(false)
const checking = ref(false)
const error = ref('')

function emptyRow(): ItsRow {
  return {
    tariffCode: '', regNumber: '', companyName: '', inn: '', kpp: '',
    workplaces: 1, activityType: '', director: '', responsible: '',
    postalCode: '', city: '', street: '', house: '', building: '', flat: '',
    phoneCode: '', phone: '', fax: '', email: '', startDate: '',
    deliveryType: '0', distributorCode: '', ownerCode: '',
  }
}

const form = ref<ItsRequest>({
  partnerCode: '00000',
  responsible: '',
  email: '',
  rows: [emptyRow()],
})

const deliveryOptions = [
  { label: 'Самовывоз', value: '0' },
  { label: 'Через дистрибьютора', value: '1' },
]

const row = computed(() => form.value.rows[0])

/** Замечания по конкретному полю: показываются прямо под ним. */
function issuesFor(field: string): ItsIssue[] {
  return issues.value.filter((issue) => issue.field === field)
}

async function check() {
  checking.value = true
  error.value = ''
  try {
    const result = await api.itsValidate(form.value)
    issues.value = result.issues ?? []
    blocking.value = result.blocking
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Не удалось проверить заявку.'
  } finally {
    checking.value = false
  }
}

/**
 * Скачивание идёт обычной формой, а не fetch: браузер сам покажет диалог
 * сохранения, а cookie сессии уйдёт вместе с запросом.
 */
async function download() {
  await check()
  if (blocking.value) return

  const response = await fetch('/api/its/download', {
    method: 'POST',
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(form.value),
  })
  if (!response.ok) {
    error.value = 'Не удалось собрать файл.'
    return
  }

  const blob = await response.blob()
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `ip${form.value.partnerCode}.xls`
  link.click()
  URL.revokeObjectURL(url)
}

onMounted(async () => {
  try {
    tariffs.value = (await api.itsTariffs()).tariffs ?? []
  } catch {
    error.value = 'Не удалось загрузить справочник тарифов.'
  }
})

// Проверяем на лету, но не на каждое нажатие клавиши.
let timer: number | undefined
watch(
  form,
  () => {
    window.clearTimeout(timer)
    timer = window.setTimeout(check, 600)
  },
  { deep: true },
)
</script>

<template>
  <section>
    <header class="bar">
      <h2>Заявка на регистрацию тарифа ЭПД</h2>
      <div class="controls">
        <Button
          label="Скачать файл"
          icon="pi pi-download"
          :disabled="blocking || checking"
          @click="download"
        />
      </div>
    </header>

    <Message v-if="error" severity="error" :closable="false">{{ error }}</Message>

    <Message v-if="blocking" severity="error" :closable="false">
      Заявку выпускать нельзя: исправьте отмеченные поля.
    </Message>
    <Message v-else-if="issues.length === 0 && !checking" severity="success" :closable="false">
      Заявка заполнена верно. Файл отправляется на itsrobot@1c.ru, протокол придёт
      с autoits@1c.ru на указанный e-mail.
    </Message>

    <div class="form">
      <fieldset>
        <legend>Отправитель</legend>
        <label>Код партнёра
          <InputText v-model="form.partnerCode" />
          <small v-for="issue in issuesFor('partnerCode')" :key="issue.message" class="bad">
            {{ issue.message }}
          </small>
        </label>
        <label>Ответственный
          <InputText v-model="form.responsible" />
        </label>
        <label>E-mail для протокола
          <InputText v-model="form.email" />
          <small v-for="issue in issuesFor('email')" :key="issue.message" class="bad">
            {{ issue.message }}
          </small>
        </label>
      </fieldset>

      <fieldset>
        <legend>Тариф</legend>
        <label>Вид 1С:ИТС
          <Select
            v-model="row.tariffCode"
            :options="tariffs"
            option-label="name"
            option-value="code"
            placeholder="Выберите тариф"
          />
          <small v-for="issue in issuesFor('tariffCode')" :key="issue.message" class="bad">
            {{ issue.message }}
          </small>
        </label>
        <label>Код абонента-владельца
          <InputText v-model="row.ownerCode" placeholder="CL-1000530" />
          <small v-for="issue in issuesFor('ownerCode')" :key="issue.message" class="bad">
            {{ issue.message }}
          </small>
        </label>
        <label>Дата начала
          <InputText v-model="row.startDate" placeholder="01.10.26" />
          <small v-for="issue in issuesFor('startDate')" :key="issue.message" class="bad">
            {{ issue.message }}
          </small>
        </label>
        <p class="fixed">
          Количество выпусков 12 и предоплата за весь срок подставляются автоматически:
          для тарифов ЭПД других значений не бывает.
        </p>
      </fieldset>

      <fieldset>
        <legend>Клиент</legend>
        <label>Наименование фирмы
          <InputText v-model="row.companyName" />
          <small v-for="issue in issuesFor('companyName')" :key="issue.message" class="bad">
            {{ issue.message }}
          </small>
        </label>
        <label>ИНН
          <InputText v-model="row.inn" />
          <small v-for="issue in issuesFor('inn')" :key="issue.message" class="bad">
            {{ issue.message }}
          </small>
        </label>
        <label>КПП
          <InputText v-model="row.kpp" />
          <small v-for="issue in issuesFor('kpp')" :key="issue.message" class="bad">
            {{ issue.message }}
          </small>
        </label>
        <label>Регистрационный номер
          <InputText v-model="row.regNumber" />
          <small v-for="issue in issuesFor('regNumber')" :key="issue.message" class="bad">
            {{ issue.message }}
          </small>
        </label>
        <label>Рабочих мест
          <InputNumber v-model="row.workplaces" :min="1" />
        </label>
        <label>Ответственный у клиента
          <InputText v-model="row.responsible" />
          <small v-for="issue in issuesFor('responsible')" :key="issue.message" class="bad">
            {{ issue.message }}
          </small>
        </label>
        <label>Код города
          <InputText v-model="row.phoneCode" />
        </label>
        <label>Телефон
          <InputText v-model="row.phone" />
          <small v-for="issue in issuesFor('phone')" :key="issue.message" class="bad">
            {{ issue.message }}
          </small>
        </label>
      </fieldset>

      <fieldset>
        <legend>Получение</legend>
        <label>Способ получения
          <Select
            v-model="row.deliveryType"
            :options="deliveryOptions"
            option-label="label"
            option-value="value"
          />
        </label>
        <label v-if="row.deliveryType === '1'">Код дистрибьютора
          <InputText v-model="row.distributorCode" />
          <small v-for="issue in issuesFor('distributorCode')" :key="issue.message" class="bad">
            {{ issue.message }}
          </small>
        </label>
      </fieldset>
    </div>
  </section>
</template>

<style scoped>
.form {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(20rem, 1fr));
  gap: 1rem;
  align-items: start;
}
fieldset {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  margin: 0;
  padding: 1rem;
  border: 1px solid var(--p-surface-300);
  border-radius: 0.5rem;
}
legend {
  padding: 0 0.5rem;
  font-weight: 600;
}
label {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 12px;
  line-height: 16px;
  color: var(--p-text-muted-color);
}
.bad {
  color: var(--p-red-500);
}
.fixed {
  margin: 0;
  font-size: 12px;
  line-height: 16px;
  color: var(--p-text-muted-color);
}
.controls {
  display: flex;
  gap: 0.75rem;
}
</style>
```

- [ ] **Step 3: Добавить маршрут и пункт меню**

В `web/src/router/index.ts`:

```ts
    { path: '/requests', name: 'requests', component: () => import('../views/RequestsView.vue') },
```

В `web/src/App.vue` в навигацию:

```html
        <RouterLink to="/requests">Заявки</RouterLink>
```

- [ ] **Step 4: Собрать**

Run: `cd web; npm run build; cd ..`
Expected: сборка без ошибок

- [ ] **Step 5: Commit**

```bash
git add web/src/ web/dist
git commit -m "feat(web): add the ITS request form with live validation"
```

---

## Task 8: Проверка вживую

- [ ] **Step 1: Собрать утилиту, если её нет**

```powershell
python -m PyInstaller --onefile --name xlsfill --distpath tools/xlsfill/dist tools/xlsfill/xlsfill.py
```

- [ ] **Step 2: Запустить сервис и открыть «Заявки»**

- [ ] **Step 3: Проверить блокировку по Фреш**

Ввести код владельца `FR-FR-600361` — должно появиться блокирующее замечание с подсказкой
кода для «Менеджера сервиса», кнопка скачивания недоступна.

- [ ] **Step 4: Проверить выпуск файла**

Заполнить корректно (владелец `CL-…`, тариф из списка, дата вида `01.10.26`), скачать файл
и открыть его: проверить, что значения стоят в нужных ячейках, а листов по-прежнему три.

---

## Самопроверка плана

**Покрытие спеки.** Раздел 4.3 закрывают задачи 1–7.

**Не входит:** отправка письма роботом (по решению заказчика файл уносит человек),
несколько строк в одной заявке через интерфейс (модель и раскладка их поддерживают,
форма показывает первую), справочник всех 449 кодов ИТС — для ЭПД нужны только 13.

**Согласованность типов.** `itsreq.Request` и `itsreq.Row` определены в Task 2 и используются
в Task 3, 4, 6. `itsreq.Cell` из Task 3 сериализуется в Task 4 в формат, который ждёт
`tools/xlsfill/xlsfill.py`. `store.RequestDraft` из Task 5 используется в Task 6.
Сигнатура `NewRouter` расширяется до пяти параметров.

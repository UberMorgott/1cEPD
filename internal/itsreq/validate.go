package itsreq

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	fiveDigits  = regexp.MustCompile(`^\d{5}$`)
	nineDigits  = regexp.MustCompile(`^\d{9}$`)
	innPattern  = regexp.MustCompile(`^\d{10}$|^\d{12}$`)
	datePattern = regexp.MustCompile(`^\d{2}\.\d{2}\.\d{2}$`)
	emailLike   = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
	// passwordPattern — пароль подлинности заявки: 5–20 латинских букв и цифр.
	passwordPattern = regexp.MustCompile(`^[A-Za-z0-9]{5,20}$`)
	// refusalReasons — коды причин отказа из справочника шаблона.
	refusalReasons = regexp.MustCompile(`^[1-5]$`)
)

// monthLayout — «Дата отказа» в формате ММ.ГГ, например 01.12.
const monthLayout = "01.06"

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
	if strings.TrimSpace(request.Responsible) == "" {
		issues = append(issues, Issue{
			Field: "responsible", Blocking: true,
			Message: "Укажите ответственного за заявку: поле шапки обязательно.",
		})
	}
	if !emailLike.MatchString(request.Email) {
		issues = append(issues, Issue{
			Field: "email", Blocking: true,
			Message: "Нужен e-mail: на него придут протокол обработки и счёт.",
		})
	}
	// Пароли необязательны, но кривой пароль робот считает подделкой заявки.
	checkPassword := func(field, value string) {
		if value != "" && !passwordPattern.MatchString(value) {
			issues = append(issues, Issue{
				Field: field, Blocking: true,
				Message: "Пароль — от 5 до 20 латинских букв и цифр.",
			})
		}
	}
	checkPassword("password", request.Password)
	checkPassword("newPassword", request.NewPassword)

	if len(request.Rows) == 0 {
		issues = append(issues, Issue{
			Field: "rows", Blocking: true,
			Message: "В заявке нет ни одной строки.",
		})
	}
	if len(request.Rows) > MaxRows {
		issues = append(issues, Issue{
			Field: "rows", Blocking: true,
			Message: fmt.Sprintf("В одном файле не больше %d строк: разбейте заявку на несколько.", MaxRows),
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
	// У ИП (ИНН из 12 цифр) КПП нет: поле остаётся пустым. У организации он обязателен.
	if len(strings.TrimSpace(row.INN)) == 12 {
		if strings.TrimSpace(row.KPP) != "" {
			add("kpp", "У ИП КПП нет: оставьте поле пустым.", false)
		}
	} else if !nineDigits.MatchString(row.KPP) {
		add("kpp", "КПП обязателен для организации и состоит из девяти цифр.", true)
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
	// E-mail строки необязателен, но кривой адрес робот не примет.
	if row.Email != "" && !emailLike.MatchString(row.Email) {
		add("email", "E-mail строки указан с ошибкой: нужен адрес вида name@example.ru.", true)
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

	switch row.operation() {
	case OperationNew:
		// Дата и причина отказа здесь просто не нужны: в файл они не попадут.
	case OperationRefusal:
		// docs/REQUESTS.md §6: для тарифов ЭПД отказы не допускаются.
		if known {
			add("operation", fmt.Sprintf(
				"Для тарифа %s отказ не оформляется: правила ЭПД допускают только новый договор или продление.",
				tariff.Name), true)
		}
		if !refusalMonthValid(row.RefusalDate) {
			add("refusalDate",
				"Дата отказа указывается в формате ММ.ГГ и только со следующего месяца, например 01.12.", true)
		}
		if !refusalReasons.MatchString(row.RefusalReason) {
			add("refusalReason", "Причина отказа — код от 1 до 5 из справочника шаблона.", true)
		}
	default:
		add("operation", "Операция: 0 — новый договор или продление, 1 — отказ.", true)
	}

	return issues
}

// refusalMonthValid проверяет «Дату отказа»: формат ММ.ГГ и не раньше
// следующего месяца — задним числом отказ не регистрируется.
func refusalMonthValid(value string) bool {
	month, err := time.Parse(monthLayout, value)
	if err != nil {
		return false
	}
	now := time.Now().UTC()
	next := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, 0)
	return !month.Before(next)
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

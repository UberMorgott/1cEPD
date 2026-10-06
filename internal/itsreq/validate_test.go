package itsreq

import (
	"strings"
	"testing"
	"time"
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

func TestHeaderResponsibleIsRequired(t *testing.T) {
	request := validRequest()
	request.Responsible = "  "

	for _, issue := range Validate(request) {
		if issue.Row == 0 && issue.Field == "responsible" && issue.Blocking {
			return
		}
	}
	t.Error("пустой ответственный в шапке должен блокировать заявку")
}

func TestRowEmailIsCheckedWhenFilled(t *testing.T) {
	request := validRequest()
	request.Rows[0].Email = "buh@firma.ru"
	if hasIssue(Validate(request), "email") {
		t.Error("верный e-mail строки не должен давать замечание")
	}

	request.Rows[0].Email = "buh@firma"
	found := false
	for _, issue := range Validate(request) {
		if issue.Row == 1 && issue.Field == "email" && issue.Blocking {
			found = true
		}
	}
	if !found {
		t.Error("кривой e-mail строки должен блокировать заявку")
	}
}

func TestRowsAreCappedByTemplate(t *testing.T) {
	request := validRequest()
	for len(request.Rows) < MaxRows {
		request.Rows = append(request.Rows, validRow())
	}
	if hasIssue(Validate(request), "rows") {
		t.Errorf("%d строк шаблон вмещает", MaxRows)
	}

	request.Rows = append(request.Rows, validRow())
	if !hasIssue(Validate(request), "rows") {
		t.Errorf("строка %d уже за оформленной частью шаблона", MaxRows+1)
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

func TestKPPRequiredForOrganization(t *testing.T) {
	request := validRequest()
	request.Rows[0].INN = "7811000310"
	request.Rows[0].KPP = ""
	if !hasIssue(Validate(request), "kpp") {
		t.Error("у организации (ИНН из 10 цифр) КПП обязателен")
	}
}

func TestKPPNotRequiredForIndividual(t *testing.T) {
	request := validRequest()
	request.Rows[0].INN = "780700000416"
	request.Rows[0].KPP = ""
	if hasIssue(Validate(request), "kpp") {
		t.Error("у ИП (ИНН из 12 цифр) КПП нет — пустое поле не ошибка")
	}

	request.Rows[0].KPP = "780001001"
	for _, issue := range Validate(request) {
		if issue.Field == "kpp" && issue.Blocking {
			t.Error("КПП у ИП — предупреждение, а не блокирующая ошибка")
		}
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

func TestRefusalRequiresDateAndReason(t *testing.T) {
	request := validRequest()
	request.Rows[0].Operation = OperationRefusal

	issues := Validate(request)

	for _, field := range []string{"refusalDate", "refusalReason"} {
		if !hasIssue(issues, field) {
			t.Errorf("при отказе поле %s обязательно: %+v", field, issues)
		}
	}
}

func TestRefusalDateOnlyFromNextMonth(t *testing.T) {
	// Задним числом и текущим месяцем отказ не регистрируется.
	now := time.Now().UTC()
	for _, bad := range []string{now.Format(monthLayout), now.AddDate(0, -1, 0).Format(monthLayout), "10.2026", ""} {
		request := validRequest()
		request.Rows[0].Operation = OperationRefusal
		request.Rows[0].RefusalReason = "1"
		request.Rows[0].RefusalDate = bad

		if !hasIssue(Validate(request), "refusalDate") {
			t.Errorf("дата отказа %q должна быть отвергнута", bad)
		}
	}
}

func TestRefusalNotAllowedForEPD(t *testing.T) {
	// docs/REQUESTS.md §6: отказы для тарифов ЭПД не допускаются.
	request := validRequest()
	request.Rows[0].Operation = OperationRefusal
	request.Rows[0].RefusalDate = nextMonthCode()
	request.Rows[0].RefusalReason = "2"

	issues := Validate(request)

	if !hasIssue(issues, "operation") || !Blocking(issues) {
		t.Errorf("отказ по тарифу ЭПД должен блокировать заявку: %+v", issues)
	}
	if hasIssue(issues, "refusalDate") || hasIssue(issues, "refusalReason") {
		t.Errorf("корректные поля отказа замечаний давать не должны: %+v", issues)
	}
}

func TestUnknownOperationIsRejected(t *testing.T) {
	request := validRequest()
	request.Rows[0].Operation = "7"

	if !hasIssue(Validate(request), "operation") {
		t.Error("операция бывает только 0 или 1")
	}
}

func TestNewOperationIgnoresRefusalFields(t *testing.T) {
	request := validRequest()
	request.Rows[0].RefusalDate = "01.12"
	request.Rows[0].RefusalReason = "9"

	if issues := Validate(request); len(issues) != 0 {
		t.Errorf("поля отказа при новом договоре просто игнорируются: %+v", issues)
	}
}

func TestHeaderPasswordFormat(t *testing.T) {
	for _, bad := range []string{"abc", "пароль12345", "with space", "0123456789012345678901"} {
		request := validRequest()
		request.Password = bad

		if !hasIssue(Validate(request), "password") {
			t.Errorf("пароль %q должен быть отвергнут", bad)
		}
	}

	request := validRequest()
	request.Password = "Secret2026"
	request.NewPassword = "Secret2027"
	if issues := Validate(request); len(issues) != 0 {
		t.Errorf("правильные пароли замечаний не дают: %+v", issues)
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

package itsreq

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// fakeDirectory — справочник 1С в памяти; реквизиты вымышлены.
type fakeDirectory struct {
	subscribers []Subscriber
	products    map[string]string
	contracts   map[string][]Contract
	logins      map[string][]string
	users       map[string]bool
	err         error
	loginCalls  int
}

func (f *fakeDirectory) Subscribers(context.Context) ([]Subscriber, error) {
	return f.subscribers, f.err
}

func (f *fakeDirectory) Products(context.Context, []string) (map[string]string, error) {
	return f.products, f.err
}

func (f *fakeDirectory) Contracts(context.Context, []string) (map[string][]Contract, error) {
	return f.contracts, f.err
}

func (f *fakeDirectory) LoginRegNumbers(_ context.Context, login string) ([]string, error) {
	f.loginCalls++
	if login == "missing@example.ru" {
		return nil, ErrLoginNotFound
	}
	return f.logins[login], f.err
}

func (f *fakeDirectory) UserExists(_ context.Context, login, email string) (bool, error) {
	return f.users[login+"|"+email], f.err
}

// directory — абонент строки validRow и посторонний абонент.
func directory() *fakeDirectory {
	return &fakeDirectory{
		subscribers: []Subscriber{
			{Code: "CL-1000530", RegNumbers: []string{"18117482", "18117483"},
				Organizations: []Organization{{Name: `ООО "Тест"`, INN: "7811000310", KPP: "780001001"}}},
			{Code: "CL-2000002", RegNumbers: []string{"18119999"},
				Organizations: []Organization{{Name: `ООО "Чужой"`, INN: "7700000009", KPP: "770001001"}}},
		},
		products:  map[string]string{"18117482": "Бухгалтерия предприятия", "18119999": "Розница"},
		contracts: map[string][]Contract{},
		logins:    map[string][]string{"client@example.ru": {"18117482"}},
		users:     map[string]bool{"client@example.ru|client@example.ru": true},
	}
}

func find(issues []Issue, field string) *Issue {
	for i := range issues {
		if issues[i].Field == field {
			return &issues[i]
		}
	}
	return nil
}

func suggested(list []Suggestion, field string) string {
	for _, s := range list {
		if s.Field == field {
			return s.Value
		}
	}
	return ""
}

func TestCrossCheckCleanRowHasNoIssues(t *testing.T) {
	request := validRequest()
	request.Rows[0].Login, request.Rows[0].Email = "client@example.ru", "client@example.ru"
	request.Rows[0].ExtraRegNumbers = []string{"18117483"}

	result := CrossCheck(context.Background(), directory(), request)
	if len(result.Issues) != 0 {
		t.Errorf("замечания: %+v", result.Issues)
	}
	if note := find(result.Notes, "regNumber"); note == nil || !strings.Contains(note.Message, "Бухгалтерия") {
		t.Errorf("нет справки о продукте: %+v", result.Notes)
	}
}

func TestCrossCheckSuggestsEmptyFields(t *testing.T) {
	request := validRequest()
	row := &request.Rows[0]
	row.CompanyName, row.KPP, row.OwnerCode, row.RegNumber = "", "", "", ""

	result := CrossCheck(context.Background(), directory(), request)
	want := map[string]string{
		"companyName": `ООО "Тест"`, "kpp": "780001001", "ownerCode": "CL-1000530",
	}
	for field, value := range want {
		if got := suggested(result.Suggestions, field); got != value {
			t.Errorf("подсказка %s = %q, ожидали %q", field, got, value)
		}
	}
	if got := suggested(result.Suggestions, "regNumber"); got != "" {
		t.Errorf("регномер подсказан из базы абонентов: %q", got)
	}
}

// Регномера абонента бывают чужих организаций (агрегатор): из базы абонентов
// их не подсказываем, даже единственный.
func TestCrossCheckDoesNotSuggestSubscriberRegNumbers(t *testing.T) {
	dir := directory()
	dir.subscribers[0].RegNumbers = []string{"18117482"}
	request := validRequest()
	request.Rows[0].RegNumber = ""
	for _, req := range []Request{request, validRequest()} {
		result := CrossCheck(context.Background(), dir, req)
		for _, field := range []string{"regNumber", "extraRegNumbers"} {
			if got := suggested(result.Suggestions, field); got != "" {
				t.Errorf("подсказка %s = %q из базы абонентов", field, got)
			}
		}
	}
}

// Два договора одного регномера с тем же названием и концом дают одну справку.
func TestCrossCheckNotesContractOnce(t *testing.T) {
	dir := directory()
	end := time.Now().AddDate(1, 0, 0)
	contract := Contract{Name: "Договор аренды ПП 1С", Start: time.Now().AddDate(-1, 0, 0), End: end}
	dir.contracts = map[string][]Contract{"18117482": {contract, contract}}

	result := CrossCheck(context.Background(), dir, validRequest())
	count := 0
	for _, note := range result.Notes {
		if strings.Contains(note.Message, "Договор аренды ПП 1С") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("справок о договоре %d, ожидали 1: %+v", count, result.Notes)
	}
}

func TestCrossCheckBlocksMismatches(t *testing.T) {
	cases := map[string]struct {
		change func(*Row)
		field  string
	}{
		"КПП не как в 1С":           {func(r *Row) { r.KPP = "781101002" }, "kpp"},
		"ИНН другого абонента":      {func(r *Row) { r.INN = "7700000009" }, "inn"},
		"регномер другого абонента": {func(r *Row) { r.RegNumber = "18119999" }, "regNumber"},
		"логина нет на Портале":     {func(r *Row) { r.Login = "missing@example.ru" }, "login"},
		"логин без этого регномера": {func(r *Row) { r.Login = "client@example.ru"; r.RegNumber = "18117483" }, "login"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			request := validRequest()
			tc.change(&request.Rows[0])
			issue := find(CrossCheck(context.Background(), directory(), request).Issues, tc.field)
			if issue == nil || !issue.Blocking {
				t.Errorf("ожидали блокирующее замечание по %s, получили %+v", tc.field, issue)
			}
		})
	}
}

func TestCrossCheckWarnings(t *testing.T) {
	cases := map[string]struct {
		change func(*Row)
		field  string
	}{
		"e-mail не как у пользователя": {func(r *Row) { r.Login = "client@example.ru"; r.Email = "other@example.ru" }, "email"},
		"ИНН не клиента партнёра":      {func(r *Row) { r.INN = "7800000001"; r.OwnerCode = "" }, "inn"},
		"неизвестный владелец":         {func(r *Row) { r.OwnerCode = "CL-9999999" }, "ownerCode"},
		"регномер без продукта":        {func(r *Row) { r.RegNumber = "18117483" }, "regNumber"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			request := validRequest()
			tc.change(&request.Rows[0])
			issue := find(CrossCheck(context.Background(), directory(), request).Issues, tc.field)
			if issue == nil || issue.Blocking || issue.Unchecked {
				t.Errorf("ожидали предупреждение по %s, получили %+v", tc.field, issue)
			}
		})
	}
}

func TestCrossCheckRenewalDate(t *testing.T) {
	code := 2083
	end := time.Now().AddDate(0, 2, 0).UTC().Truncate(24 * time.Hour).Add(21*time.Hour - time.Second)
	dir := directory()
	dir.contracts["18117482"] = []Contract{{Name: "1С-ЭДО. ЭПД-10000", TypeNumber: &code, End: end}}
	next := end.Add(time.Second).In(moscow).Format("02.01.06")

	// Дата начала внутри действующего тарифа того же вида — дубль регистрации.
	request := validRequest()
	issue := find(CrossCheck(context.Background(), dir, request).Issues, "startDate")
	if issue == nil || !issue.Blocking || !strings.Contains(issue.Message, next) {
		t.Fatalf("ожидали блок с датой %s, получили %+v", next, issue)
	}

	// Другой тариф ЭПД поверх действующего — предупреждение, не блок.
	request.Rows[0].TariffCode = "2084"
	if issue := find(CrossCheck(context.Background(), dir, request).Issues, "startDate"); issue == nil || issue.Blocking {
		t.Errorf("смена тарифа: %+v", issue)
	}

	// Пустая дата — подсказка «день после окончания».
	request.Rows[0].StartDate = ""
	if got := suggested(CrossCheck(context.Background(), dir, request).Suggestions, "startDate"); got != next {
		t.Errorf("подсказка даты %q, ожидали %q", got, next)
	}

	// Дата после окончания — продление, замечаний нет.
	request.Rows[0].StartDate = next
	if issue := find(CrossCheck(context.Background(), dir, request).Issues, "startDate"); issue != nil {
		t.Errorf("продление с верной датой: %+v", issue)
	}
}

func TestCrossCheckPortalDownIsUnchecked(t *testing.T) {
	dir := directory()
	dir.err = errors.New("timeout")
	request := validRequest()
	request.Rows[0].Login, request.Rows[0].Email = "client@example.ru", "client@example.ru"

	result := CrossCheck(context.Background(), dir, request)
	if Blocking(result.Issues) || !Unchecked(result.Issues) {
		t.Errorf("недоступная 1С должна давать «не проверено»: %+v", result.Issues)
	}
}

func TestCheckSkipsPreflightWhenCrossCheckBlocks(t *testing.T) {
	request := validRequest()
	request.Rows[0].KPP = "781101002"
	// Портал nil дал бы замечание «проверки не выполнены» — его быть не должно.
	result := Check(context.Background(), nil, directory(), request)
	if !Blocking(result.Issues) || find(result.Issues, "rows") != nil {
		t.Errorf("замечания: %+v", result.Issues)
	}
}

func TestCrossCheckAsksLoginOnce(t *testing.T) {
	dir := directory()
	request := validRequest()
	request.Rows[0].Login = "client@example.ru"
	request.Rows = append(request.Rows, request.Rows[0])
	CrossCheck(context.Background(), dir, request)
	if dir.loginCalls != 1 {
		t.Errorf("логин проверен %d раз, ожидали 1", dir.loginCalls)
	}
}

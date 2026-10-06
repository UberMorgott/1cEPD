package itsreq

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"
)

// crossCheckTimeout — окно на все справочные запросы одной сверки.
const crossCheckTimeout = 20 * time.Second

// moscow — договоры 1С кончаются в московскую полночь, дата начала заявки
// пишется по Москве. Фиксированный сдвиг: tzdata на Windows нет.
var moscow = time.FixedZone("MSK", 3*60*60)

// Subscriber — абонент партнёра в 1С: код, регномера и организации.
type Subscriber struct {
	Code          string
	Name          string
	RegNumbers    []string
	Organizations []Organization
}

// Organization — организация абонента в 1С.
type Organization struct {
	Name string
	INN  string
	KPP  string
}

// Contract — договор 1С:ИТС или сервиса у регистрационного номера.
type Contract struct {
	Name string
	// TypeNumber — publicSubscriptionTypeNumber; для тарифа ЭПД совпадает с его кодом.
	TypeNumber *int
	Start      time.Time
	End        time.Time
}

// Directory — справочные данные 1С, с которыми сверяется заявка
// (docs/API.md §5, §7). Все методы только читают.
type Directory interface {
	// Subscribers — вся база абонентов партнёра.
	Subscribers(ctx context.Context) ([]Subscriber, error)
	// Products — наименование продукта по регномеру; неизвестных регномеров в ответе нет.
	Products(ctx context.Context, regNumbers []string) (map[string]string, error)
	// Contracts — договоры по регномеру (checkItsByRegNum).
	Contracts(ctx context.Context, regNumbers []string) (map[string][]Contract, error)
	// LoginRegNumbers — регномера программ Личного кабинета с этим логином.
	LoginRegNumbers(ctx context.Context, login string) ([]string, error)
	// UserExists — есть ли пользователь с таким логином и e-mail одновременно.
	UserExists(ctx context.Context, login, email string) (bool, error)
}

// ErrLoginNotFound — Directory.LoginRegNumbers: логина на Портале ИТС нет.
var ErrLoginNotFound = errors.New("логин не найден на Портале ИТС")

// Suggestion — значение из 1С для пустого поля строки.
type Suggestion struct {
	Row   int    `json:"row"`
	Field string `json:"field"`
	// Value — строка; для extraRegNumbers — регномера через запятую.
	Value string `json:"value"`
}

// Result — итог проверки заявки: замечания, справки из 1С и подсказки.
type Result struct {
	Issues []Issue `json:"issues"`
	// Notes — сведения из 1С для человека: продукт по регномеру, действующий договор.
	Notes       []Issue      `json:"notes"`
	Suggestions []Suggestion `json:"suggestions"`
}

// Check проверяет заявку целиком: формат, сверка с данными 1С (directory)
// и проверки в 1С (portal). Отчёт трафика в Preflight дорогой — он расходует
// часовой лимит задач 1С, поэтому идёт последним и только без блокирующих замечаний.
func Check(ctx context.Context, portal Portal, directory Directory, request Request) Result {
	result := Result{Issues: Validate(request), Notes: []Issue{}, Suggestions: []Suggestion{}}
	if directory != nil {
		cross := CrossCheck(ctx, directory, request)
		result.Issues = append(result.Issues, cross.Issues...)
		result.Notes, result.Suggestions = cross.Notes, cross.Suggestions
	}
	if Blocking(result.Issues) {
		return result
	}
	result.Issues = append(result.Issues, Preflight(ctx, portal, request)...)
	return result
}

// crossCheck — состояние одной сверки: ответы 1С и накопленный итог.
type crossCheck struct {
	result      Result
	subscribers []Subscriber
	products    map[string]string
	contracts   map[string][]Contract
	// loginRegs и users — кэш на прогон: одна строка — один вызов на значение.
	loginRegs map[string]lookup[[]string]
	users     map[string]lookup[bool]
}

type lookup[T any] struct {
	value T
	err   error
}

// CrossCheck сверяет строки заявки с базой абонентов, номенклатурой, договорами
// и пользователями Портала — то, на чём робот 1С бракует заявку чаще всего, —
// и подсказывает значения для пустых полей. Недоступность 1С заявку не
// бракует: такие замечания приходят с Unchecked.
func CrossCheck(ctx context.Context, directory Directory, request Request) Result {
	ctx, cancel := context.WithTimeout(ctx, crossCheckTimeout)
	defer cancel()

	c := &crossCheck{
		result:    Result{Notes: []Issue{}, Suggestions: []Suggestion{}},
		loginRegs: map[string]lookup[[]string]{}, users: map[string]lookup[bool]{},
	}

	subscribers, err := directory.Subscribers(ctx)
	if err != nil {
		slog.Warn("база абонентов 1С не получена", "err", err)
		c.unchecked(0, "rows", "Сверка с базой абонентов 1С не выполнена: 1С не ответила.")
	}
	c.subscribers = subscribers

	var regNumbers []string
	for _, row := range request.Rows {
		if number := strings.TrimSpace(row.RegNumber); isNumber(number) && !slices.Contains(regNumbers, number) {
			regNumbers = append(regNumbers, number)
		}
	}
	if len(regNumbers) > 0 {
		if c.products, err = directory.Products(ctx, regNumbers); err != nil {
			slog.Warn("номенклатура по регномерам не получена", "err", err)
			c.products = nil
		}
		if c.contracts, err = directory.Contracts(ctx, regNumbers); err != nil {
			slog.Warn("договоры по регномерам не получены", "err", err)
			c.contracts = nil
		}
	}

	for index, row := range request.Rows {
		c.row(ctx, directory, index+1, row)
	}
	return c.result
}

func (c *crossCheck) add(row int, field, message string, blocking bool) {
	c.result.Issues = append(c.result.Issues, Issue{Row: row, Field: field, Message: message, Blocking: blocking})
}

func (c *crossCheck) unchecked(row int, field, message string) {
	c.result.Issues = append(c.result.Issues, Issue{Row: row, Field: field, Message: message, Unchecked: true})
}

// note добавляет справку. Разные договоры одного регномера (1С отдаёт их
// отдельными записями) дают одинаковую строку «действует … до …» — повтор не нужен.
func (c *crossCheck) note(row int, field, message string) {
	note := Issue{Row: row, Field: field, Message: message}
	if !slices.Contains(c.result.Notes, note) {
		c.result.Notes = append(c.result.Notes, note)
	}
}

func (c *crossCheck) suggest(row int, field, value string) {
	if value = strings.TrimSpace(value); value != "" {
		c.result.Suggestions = append(c.result.Suggestions, Suggestion{Row: row, Field: field, Value: value})
	}
}

func (c *crossCheck) row(ctx context.Context, directory Directory, number int, row Row) {
	inn, kpp := strings.TrimSpace(row.INN), strings.TrimSpace(row.KPP)
	regNumber := strings.TrimSpace(row.RegNumber)
	owner := strings.TrimSpace(row.OwnerCode)

	if c.subscribers != nil {
		c.subscriber(number, row, inn, kpp, regNumber, owner)
	}
	if isNumber(regNumber) {
		c.product(number, regNumber)
		c.contract(number, row, regNumber)
	}
	c.login(ctx, directory, number, row, regNumber)
}

// subscriber сверяет ИНН, КПП, код владельца и регномер с базой абонентов
// и подсказывает пустые реквизиты.
func (c *crossCheck) subscriber(number int, row Row, inn, kpp, regNumber, owner string) {
	byOwner, ownerKnown := c.byCode(owner)
	if owner != "" && !ownerKnown {
		c.add(number, "ownerCode", fmt.Sprintf(
			"Абонента %s нет среди клиентов партнёра в 1С: проверьте код владельца.", owner), false)
	}

	matched := byOwner
	if !ownerKnown {
		matched = c.pick(inn, regNumber)
	}

	if inn != "" {
		holders := c.byINN(inn)
		switch {
		case len(holders) == 0:
			c.add(number, "inn", fmt.Sprintf(
				"ИНН %s нет среди организаций клиентов партнёра в 1С: связь с клиентом по ЭДО в Кабинете партнёра не настроена "+
					"или ИНН указан с ошибкой.", inn), false)
		case ownerKnown && !hasINN(byOwner, inn):
			c.add(number, "inn", fmt.Sprintf(
				"ИНН %s в 1С принадлежит абоненту %s, а не владельцу %s. Робот сверяет реквизиты с 1С.",
				inn, codes(holders), owner), true)
		}

		if kpp != "" {
			if known := c.kpps(matched, inn); len(known) > 0 && !slices.Contains(known, kpp) {
				c.add(number, "kpp", fmt.Sprintf(
					"КПП %s не совпадает с 1С: у ИНН %s там %s. Продление не меняет ИНН и КПП.",
					kpp, inn, strings.Join(known, ", ")), true)
			}
		} else if known := c.kpps(matched, inn); len(known) == 1 {
			c.suggest(number, "kpp", known[0])
		}

		if strings.TrimSpace(row.CompanyName) == "" {
			c.suggest(number, "companyName", c.orgName(matched, inn, kpp))
		}
	} else if matched != nil && len(matched.Organizations) == 1 {
		org := matched.Organizations[0]
		c.suggest(number, "inn", org.INN)
		if kpp == "" {
			c.suggest(number, "kpp", org.KPP)
		}
		if strings.TrimSpace(row.CompanyName) == "" {
			c.suggest(number, "companyName", org.Name)
		}
	}

	if regNumber != "" && matched != nil && matched.Code != "" && !slices.Contains(matched.RegNumbers, regNumber) {
		if others := c.byRegNumber(regNumber); len(others) > 0 {
			c.add(number, "regNumber", fmt.Sprintf(
				"Регистрационный номер %s в 1С записан за абонентом %s, а не %s.",
				regNumber, codes(others), matched.Code), true)
		}
	}

	if matched == nil {
		return
	}
	// Регномера абонента не подсказываем: у агрегатора (облако Фреш, бухгалтер
	// на много фирм) они чужих организаций. Регномера клиента форма берёт из его
	// программ и прошлых заявок.
	if owner == "" {
		c.suggest(number, "ownerCode", matched.Code)
	}
}

// pick находит абонента строки без кода владельца: по ИНН, а при нескольких
// абонентах с этим ИНН — по регномеру. Неоднозначность — nil.
func (c *crossCheck) pick(inn, regNumber string) *Subscriber {
	candidates := c.byINN(inn)
	if len(candidates) == 0 && regNumber != "" {
		candidates = c.byRegNumber(regNumber)
	}
	if len(candidates) > 1 && regNumber != "" {
		var narrowed []*Subscriber
		for _, s := range candidates {
			if slices.Contains(s.RegNumbers, regNumber) {
				narrowed = append(narrowed, s)
			}
		}
		candidates = narrowed
	}
	if len(candidates) == 1 {
		return candidates[0]
	}
	return nil
}

func (c *crossCheck) byCode(code string) (*Subscriber, bool) {
	if code == "" {
		return nil, false
	}
	for i := range c.subscribers {
		if strings.EqualFold(c.subscribers[i].Code, code) {
			return &c.subscribers[i], true
		}
	}
	return nil, false
}

func (c *crossCheck) byINN(inn string) []*Subscriber {
	var list []*Subscriber
	if inn == "" {
		return list
	}
	for i := range c.subscribers {
		if hasINN(&c.subscribers[i], inn) {
			list = append(list, &c.subscribers[i])
		}
	}
	return list
}

func (c *crossCheck) byRegNumber(regNumber string) []*Subscriber {
	var list []*Subscriber
	for i := range c.subscribers {
		if slices.Contains(c.subscribers[i].RegNumbers, regNumber) {
			list = append(list, &c.subscribers[i])
		}
	}
	return list
}

// kpps — непустые КПП организации с этим ИНН: у абонента строки, а если он
// не определён — у всех абонентов. Пустой КПП в 1С ничего не доказывает.
func (c *crossCheck) kpps(matched *Subscriber, inn string) []string {
	scope := c.byINN(inn)
	if matched != nil {
		scope = []*Subscriber{matched}
	}
	var list []string
	for _, s := range scope {
		for _, org := range s.Organizations {
			if org.INN == inn && org.KPP != "" && !slices.Contains(list, org.KPP) {
				list = append(list, org.KPP)
			}
		}
	}
	return list
}

func (c *crossCheck) orgName(matched *Subscriber, inn, kpp string) string {
	scope := c.byINN(inn)
	if matched != nil {
		scope = []*Subscriber{matched}
	}
	name := ""
	for _, s := range scope {
		for _, org := range s.Organizations {
			if org.INN != inn || org.Name == "" {
				continue
			}
			if kpp != "" && org.KPP == kpp {
				return org.Name
			}
			if name == "" {
				name = org.Name
			}
		}
	}
	return name
}

// product показывает продукт регномера; незнакомый номенклатуре регномер —
// повод перепроверить цифры.
func (c *crossCheck) product(number int, regNumber string) {
	if c.products == nil {
		c.unchecked(number, "regNumber", "Продукт по регистрационному номеру не проверен: 1С не ответила.")
		return
	}
	name, ok := c.products[regNumber]
	if !ok {
		c.add(number, "regNumber", fmt.Sprintf(
			"1С не знает продукта с регистрационным номером %s: проверьте цифры.", regNumber), false)
		return
	}
	if name != "" {
		c.note(number, "regNumber", "Продукт по регномеру: "+name+".")
	}
}

// contract отличает продление от нового договора: если по регномеру уже
// действует тариф ЭПД, дата начала должна идти после его окончания.
func (c *crossCheck) contract(number int, row Row, regNumber string) {
	if c.contracts == nil {
		c.unchecked(number, "startDate", "Действующие договоры по регномеру не проверены: 1С не ответила.")
		return
	}
	start, startErr := time.ParseInLocation("02.01.06", strings.TrimSpace(row.StartDate), moscow)
	now := time.Now().UTC()

	var active *Contract
	for i, contract := range c.contracts[regNumber] {
		if contract.End.IsZero() || contract.End.Before(now) {
			continue
		}
		if contract.TypeNumber != nil {
			if _, epd := TariffByCode(strconv.Itoa(*contract.TypeNumber)); epd {
				if active == nil || contract.End.After(active.End) {
					active = &c.contracts[regNumber][i]
				}
				continue
			}
		}
		c.note(number, "startDate", fmt.Sprintf("По регномеру действует %s до %s.",
			firstText(contract.Name, "договор 1С:ИТС"), contract.End.In(moscow).Format("02.01.06")))
	}
	if active == nil {
		return
	}

	next := active.End.Add(time.Second).In(moscow)
	if startErr != nil {
		if strings.TrimSpace(row.StartDate) == "" {
			c.suggest(number, "startDate", next.Format("02.01.06"))
		}
		return
	}
	if !start.Before(time.Date(next.Year(), next.Month(), next.Day(), 0, 0, 0, 0, moscow)) {
		c.note(number, "startDate", fmt.Sprintf("Это продление: текущий тариф ЭПД действует до %s.",
			active.End.In(moscow).Format("02.01.06")))
		return
	}
	same := strconv.Itoa(*active.TypeNumber) == strings.TrimSpace(row.TariffCode)
	message := fmt.Sprintf("По регномеру уже действует %s до %s: это продление, дата начала — не раньше %s.",
		firstText(active.Name, "тариф ЭПД"), active.End.In(moscow).Format("02.01.06"), next.Format("02.01.06"))
	c.add(number, "startDate", message, same)
}

// login сверяет логин Личного кабинета с регномером и e-mail строки.
func (c *crossCheck) login(ctx context.Context, directory Directory, number int, row Row, regNumber string) {
	login := strings.TrimSpace(row.Login)
	if login == "" {
		return
	}

	regs, done := c.loginRegs[strings.ToLower(login)]
	if !done {
		regs.value, regs.err = directory.LoginRegNumbers(ctx, login)
		c.loginRegs[strings.ToLower(login)] = regs
	}
	switch {
	case errors.Is(regs.err, ErrLoginNotFound):
		c.add(number, "login", fmt.Sprintf(
			"Логин %s не найден на Портале ИТС: тариф не на что регистрировать.", login), true)
	case regs.err != nil:
		slog.Warn("программы по логину не получены", "err", regs.err)
		c.unchecked(number, "login", "Кабинет по логину не проверен: 1С не ответила.")
	case len(regs.value) == 0:
		c.add(number, "login", fmt.Sprintf(
			"В Личном кабинете с логином %s нет ни одной программы: проверьте логин.", login), false)
	case regNumber != "" && !slices.Contains(regs.value, regNumber):
		c.add(number, "login", fmt.Sprintf(
			"Регномер %s не заведён в Личном кабинете с логином %s: логин принадлежит другому клиенту "+
				"или указан не тот регномер.", regNumber, login), true)
	}

	email := strings.TrimSpace(row.Email)
	if email == "" || !emailLike.MatchString(email) {
		return
	}
	key := strings.ToLower(login + "\x00" + email)
	found, done := c.users[key]
	if !done {
		found.value, found.err = directory.UserExists(ctx, login, email)
		c.users[key] = found
	}
	switch {
	case found.err != nil:
		slog.Warn("пользователь Портала не проверен", "err", found.err)
		c.unchecked(number, "email", "E-mail не сверен с Порталом ИТС: 1С не ответила.")
	case !found.value:
		c.add(number, "email", fmt.Sprintf(
			"E-mail %s не совпадает с e-mail пользователя %s на Портале ИТС.", email, login), false)
	}
}

func hasINN(s *Subscriber, inn string) bool {
	for _, org := range s.Organizations {
		if org.INN == inn {
			return true
		}
	}
	return false
}

func codes(list []*Subscriber) string {
	names := make([]string, 0, len(list))
	for _, s := range list {
		names = append(names, s.Code)
	}
	return strings.Join(names, ", ")
}

func isNumber(value string) bool {
	_, err := strconv.ParseUint(value, 10, 63)
	return err == nil
}

func firstText(value, fallback string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return fallback
}

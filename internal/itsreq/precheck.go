package itsreq

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// Таймауты на одну проверку. Отчёт трафика 1С строит задачей и отдаёт не сразу,
// поэтому окно у него шире, чем у синхронного поиска по регномеру.
const (
	regNumberCheckTimeout = 15 * time.Second
	edoTrafficTimeout     = 90 * time.Second
	// edoTrafficWindow — глубина отчёта трафика. Год берём, чтобы поймать связь
	// клиента, у которого документооборот был давно.
	edoTrafficWindow = 365 * 24 * time.Hour
)

// Portal — то, что проверкам перед выпуском нужно от партнёрского API 1С.
// Интерфейс объявлен здесь, а не в partner, чтобы itsreq не тянул за собой
// весь клиент; *partner.Client ему удовлетворяет.
type Portal interface {
	RegNumberRegistered(ctx context.Context, regNumber string) (bool, error)
	EDOTrafficLogins(ctx context.Context, inn string, from, to time.Time) ([]string, error)
}

// errNotRegistered — приговор проверки, а не сбой связи. Отличать обязательно:
// при недоступной 1С заявка не становится плохой, и блокировать её нельзя.
var errNotRegistered = errors.New("регномер не найден в личном кабинете")

// ValidateWithPortal проверяет заявку сначала по формату, а затем — если формат
// в порядке — обращается в 1С за тем, что офлайн не узнать.
//
// Порядок принципиален: форматная проверка бесплатна, а каждый отчёт трафика
// расходует часовой лимит задач 1С.
func ValidateWithPortal(ctx context.Context, portal Portal, request Request) []Issue {
	return Check(ctx, portal, nil, request).Issues
}

// Preflight выполняет проверки, требующие 1С (docs/REQUESTS.md §6):
// регистрационный номер заведён в Личном кабинете клиента, а идентификатор ЭДО
// связан с логином этого кабинета.
//
// Недоступность API — не повод забраковать заявку. Такие замечания приходят
// с Unchecked, не блокируют выпуск и пишутся в лог предупреждением.
func Preflight(ctx context.Context, portal Portal, request Request) []Issue {
	var issues []Issue
	add := func(row int, field, message string, blocking, unchecked bool) {
		issues = append(issues, Issue{Row: row, Field: field, Message: message,
			Blocking: blocking, Unchecked: unchecked})
	}

	if portal == nil {
		add(0, "rows", "Проверки в 1С не выполнены: партнёрское API не настроено.", false, true)
		return issues
	}

	// Один прогон — один вызов на уникальное значение: повторный отчёт по тому же
	// клиенту впустую тратит лимит задач 1С.
	regCache := make(map[string]error)
	trafficCache := make(map[string]trafficResult)

	for index, row := range request.Rows {
		number := index + 1

		if regNumber := strings.TrimSpace(row.RegNumber); regNumber != "" {
			err, done := regCache[regNumber]
			if !done {
				err = checkRegNumber(ctx, portal, regNumber)
				regCache[regNumber] = err
			}
			switch {
			case err == nil:
			case errors.Is(err, errNotRegistered):
				add(number, "regNumber", fmt.Sprintf(
					"Регистрационный номер %s не зарегистрирован в Личном кабинете клиента на Портале ИТС.",
					regNumber), true, false)
			default:
				add(number, "regNumber",
					"Регистрационный номер не проверен: 1С не ответила. "+err.Error(), false, true)
			}
		}

		issues = append(issues, checkEDOLink(ctx, portal, row, number, trafficCache)...)
	}
	return issues
}

// trafficResult — разобранный отчёт трафика по одному ИНН либо ошибка обращения.
type trafficResult struct {
	logins []string
	err    error
}

func checkRegNumber(ctx context.Context, portal Portal, regNumber string) error {
	ctx, cancel := context.WithTimeout(ctx, regNumberCheckTimeout)
	defer cancel()

	registered, err := portal.RegNumberRegistered(ctx, regNumber)
	if err != nil {
		slog.Warn("регномер не проверен в 1С", "regNumber", regNumber, "err", err)
		return err
	}
	if !registered {
		return errNotRegistered
	}
	return nil
}

// checkEDOLink проверяет, что идентификатор ЭДО клиента связан с логином его
// Личного кабинета. Источник — отчёт трафика ЭДО по ИНН.
func checkEDOLink(ctx context.Context, portal Portal, row Row, number int,
	cache map[string]trafficResult,
) []Issue {
	login := strings.TrimSpace(row.Login)
	inn := strings.TrimSpace(row.INN)
	if login == "" || inn == "" {
		return []Issue{{Row: number, Field: "login", Unchecked: true,
			Message: "Связь идентификатора ЭДО не проверена: не указан логин Личного кабинета клиента."}}
	}

	result, done := cache[inn]
	if !done {
		result = loadTrafficLogins(ctx, portal, inn)
		cache[inn] = result
	}

	if result.err != nil {
		return []Issue{{Row: number, Field: "login", Unchecked: true,
			Message: "Связь идентификатора ЭДО не проверена: 1С не ответила. " + result.err.Error()}}
	}

	for _, found := range result.logins {
		if strings.EqualFold(found, login) {
			return nil
		}
	}

	// Пустой отчёт означает «документооборота не было», а не «связи нет»:
	// у нового клиента трафика может ещё не быть. Не блокируем.
	if len(result.logins) == 0 {
		return []Issue{{Row: number, Field: "login", Unchecked: true,
			Message: "Связь идентификатора ЭДО подтвердить не удалось: по клиенту нет трафика ЭДО. " +
				"Проверьте вручную в Кабинете партнёра."}}
	}

	return []Issue{{Row: number, Field: "login", Blocking: true,
		Message: fmt.Sprintf(
			"Идентификатор ЭДО не связан с логином %s. Тариф регистрируется на тот Личный кабинет, "+
				"к логину которого привязан идентификатор.", login)}}
}

func loadTrafficLogins(ctx context.Context, portal Portal, inn string) trafficResult {
	ctx, cancel := context.WithTimeout(ctx, edoTrafficTimeout)
	defer cancel()

	to := time.Now().UTC()
	logins, err := portal.EDOTrafficLogins(ctx, inn, to.Add(-edoTrafficWindow), to)
	if err != nil {
		slog.Warn("связь идентификатора ЭДО не проверена в 1С", "inn", inn, "err", err)
		return trafficResult{err: err}
	}
	return trafficResult{logins: logins}
}

// Unchecked сообщает, что хотя бы одну проверку выполнить не удалось.
func Unchecked(issues []Issue) bool {
	for _, issue := range issues {
		if issue.Unchecked {
			return true
		}
	}
	return false
}

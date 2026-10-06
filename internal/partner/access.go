package partner

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// RegNumberRegistered сообщает, знает ли Личный кабинет клиента такой
// регистрационный номер программы. См. docs/API.md §7.
//
// Ответ — массив программ; нас интересует только его непустота. Разбирать
// элементы нельзя: в запросе регномер число, а тип поля в ответе документация
// описывает противоречиво (docs/API.md §12, грабля 3).
func (c *Client) RegNumberRegistered(ctx context.Context, regNumber string) (bool, error) {
	number, err := strconv.ParseInt(strings.TrimSpace(regNumber), 10, 64)
	if err != nil {
		return false, fmt.Errorf("partner: регистрационный номер %q не число: %w", regNumber, err)
	}

	body, err := json.Marshal(map[string]int64{"regNumber": number})
	if err != nil {
		return false, fmt.Errorf("partner: не собрать тело запроса: %w", err)
	}

	resp, err := c.post(ctx, "/client-program-access/search/reg-number", body)
	if err != nil {
		return false, fmt.Errorf("partner: не проверить регномер: %w", err)
	}

	var list []json.RawMessage
	if err := json.Unmarshal(resp.Body, &list); err != nil {
		return false, fmt.Errorf("partner: не разобрать ответ о доступе к программам: %w", err)
	}
	return len(list) > 0, nil
}

// ProgramAccess — строка страницы «Проверка условий сопровождения» Личного
// кабинета: регномер, программа и выполнены ли условия сопровождения.
type ProgramAccess struct {
	RegNumber string
	Program   string
	HasAccess bool
	// MissingConditions — названия недостающих условий; пусто, если выполнены.
	MissingConditions []string
}

// ProgramsByLogin возвращает программы Личного кабинета с этим логином.
// См. docs/API.md §7 «Доступ к программам».
func (c *Client) ProgramsByLogin(ctx context.Context, login string) ([]ProgramAccess, error) {
	body, err := json.Marshal(map[string]string{"login": strings.TrimSpace(login)})
	if err != nil {
		return nil, fmt.Errorf("partner: не собрать тело запроса: %w", err)
	}
	return c.programAccess(ctx, "/client-program-access/search/login", body)
}

// ProgramsByRegNumber возвращает программы кабинета, где заведён этот регномер.
func (c *Client) ProgramsByRegNumber(ctx context.Context, regNumber string) ([]ProgramAccess, error) {
	number, err := strconv.ParseInt(strings.TrimSpace(regNumber), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("partner: регистрационный номер %q не число: %w", regNumber, err)
	}
	body, err := json.Marshal(map[string]int64{"regNumber": number})
	if err != nil {
		return nil, fmt.Errorf("partner: не собрать тело запроса: %w", err)
	}
	return c.programAccess(ctx, "/client-program-access/search/reg-number", body)
}

func (c *Client) programAccess(ctx context.Context, path string, body []byte) ([]ProgramAccess, error) {
	resp, err := c.post(ctx, path, body)
	if err != nil {
		return nil, fmt.Errorf("partner: не проверить условия сопровождения: %w", err)
	}
	return ParseProgramAccess(resp.Body)
}

// ParseProgramAccess разбирает ответ client-program-access.
//
// Регномер в ответе бывает и числом, и строкой (docs/API.md §12, грабля 3),
// поэтому берём его сырым и снимаем кавычки.
func ParseProgramAccess(raw []byte) ([]ProgramAccess, error) {
	var items []struct {
		RegNumber json.RawMessage `json:"regNumber"`
		Program   struct {
			Name string `json:"name"`
		} `json:"program"`
		HasAccess         bool `json:"hasAccess"`
		MissingConditions []struct {
			Name string `json:"name"`
		} `json:"missingSupportConditions"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("partner: не разобрать ответ о доступе к программам: %w", err)
	}

	list := make([]ProgramAccess, 0, len(items))
	for _, item := range items {
		access := ProgramAccess{
			RegNumber: strings.Trim(strings.TrimSpace(string(item.RegNumber)), `"`),
			Program:   item.Program.Name,
			HasAccess: item.HasAccess,
		}
		if access.RegNumber == "null" {
			access.RegNumber = ""
		}
		for _, condition := range item.MissingConditions {
			if name := strings.TrimSpace(condition.Name); name != "" {
				access.MissingConditions = append(access.MissingConditions, name)
			}
		}
		list = append(list, access)
	}
	return list, nil
}

// EDOTrafficLogins возвращает логины клиента, у которых в отчёте трафика ЭДО
// есть непустой идентификатор ЭДО. См. docs/API.md §8.2.
//
// Фильтруем только по ИНН: отчёт, суженный до одного логина, не позволил бы
// отличить «логин без связи» от «связь есть, но у другого логина».
//
// ponytail: отчёт видит только тех, у кого был документооборот, поэтому пустой
// результат означает «нет данных», а не «связи нет». Разбирать это должен
// вызывающий; отдельного источника связей в партнёрском API нет.
func (c *Client) EDOTrafficLogins(ctx context.Context, inn string, from, to time.Time) ([]string, error) {
	raw, err := c.clientTrafficReport(ctx, map[string]string{
		"inn":        strings.TrimSpace(inn),
		"periodFrom": from.UTC().Format(time.RFC3339),
		"periodTo":   to.UTC().Format(time.RFC3339),
	})
	if err != nil {
		return nil, err
	}
	return ParseEDOTrafficLogins(raw)
}

// EDOTrafficReport строит отчёт трафика ЭДО по всем клиентам партнёра за
// период — без фильтра по клиенту 1С его строит (docs/API.md §8.2). Возвращает
// сырой CSV для ParseEDOTraffic. Повторно ставить задачу при ошибке нельзя:
// это расходует часовой лимит отчётов.
func (c *Client) EDOTrafficReport(ctx context.Context, from, to time.Time) ([]byte, error) {
	return c.clientTrafficReport(ctx, map[string]string{
		"periodFrom": from.UTC().Format(time.RFC3339),
		"periodTo":   to.UTC().Format(time.RFC3339),
	})
}

// clientTrafficReport ставит задачу на отчёт трафика и ждёт готовый CSV.
func (c *Client) clientTrafficReport(ctx context.Context, filter map[string]string) ([]byte, error) {
	body, err := json.Marshal(filter)
	if err != nil {
		return nil, fmt.Errorf("partner: не собрать тело запроса: %w", err)
	}

	resp, err := c.post(ctx, "/edo/reports/client-traffic", body)
	if err != nil {
		return nil, fmt.Errorf("partner: не поставить задачу на отчёт трафика: %w", err)
	}

	var task taskResponse
	if err := json.Unmarshal(resp.Body, &task); err != nil {
		return nil, fmt.Errorf("partner: не разобрать ответ на постановку задачи: %w", err)
	}
	if task.Error != "" {
		return nil, fmt.Errorf("partner: 1С отказала в построении отчёта трафика: %s", task.Error)
	}
	if task.TaskUeid == "" {
		return nil, fmt.Errorf("partner: 1С не вернула идентификатор задачи")
	}

	return c.waitForReport(ctx, "/edo/reports/client-traffic/"+task.TaskUeid)
}

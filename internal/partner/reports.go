package partner

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// taskResponse — ответ на постановку задачи построения отчёта ЭДО.
type taskResponse struct {
	TaskUeid string `json:"taskUeid"`
	Error    string `json:"error"`
}

// EDOBillingReport ставит задачу на отчёт биллинга ЭДО за указанный месяц и ждёт результат.
// Возвращает сырой CSV: разбирать его должен вызывающий через ParseEDOBilling.
//
// Повторно ставить задачу при ошибке нельзя — 1С создаст дубль и потратит часовой лимит.
func (c *Client) EDOBillingReport(ctx context.Context, date time.Time) ([]byte, error) {
	body, err := json.Marshal(map[string]string{
		"date": date.UTC().Format(time.RFC3339),
	})
	if err != nil {
		return nil, fmt.Errorf("partner: не собрать тело запроса: %w", err)
	}

	resp, err := c.post(ctx, "/edo/reports/billing", body)
	if err != nil {
		return nil, fmt.Errorf("partner: не поставить задачу на отчёт: %w", err)
	}

	var task taskResponse
	if err := json.Unmarshal(resp.Body, &task); err != nil {
		return nil, fmt.Errorf("partner: не разобрать ответ на постановку задачи: %w", err)
	}
	if task.Error != "" {
		return nil, fmt.Errorf("partner: 1С отказала в построении отчёта: %s", task.Error)
	}
	if task.TaskUeid == "" {
		return nil, fmt.Errorf("partner: 1С не вернула идентификатор задачи")
	}

	return c.waitForReport(ctx, "/edo/reports/billing/"+task.TaskUeid)
}

// waitForReport опрашивает готовность отчёта.
//
// Пока отчёт строится, 1С отвечает 204 либо 202 — проверено живыми вызовами,
// оба кода встречаются на одном и том же эндпоинте. Готовый отчёт приходит
// как 200 с непустым телом.
func (c *Client) waitForReport(ctx context.Context, path string) ([]byte, error) {
	for {
		resp, err := c.get(ctx, path)
		if err != nil {
			return nil, fmt.Errorf("partner: не забрать отчёт: %w", err)
		}
		switch {
		case resp.StatusCode == http.StatusNoContent, resp.StatusCode == http.StatusAccepted:
			// Отчёт ещё строится — ждём следующего опроса.
		case resp.StatusCode == http.StatusOK && len(resp.Body) > 0:
			return resp.Body, nil
		case resp.StatusCode == http.StatusOK:
			return nil, fmt.Errorf("partner: 1С вернула пустой отчёт")
		default:
			return nil, fmt.Errorf("partner: неожиданный код %d при получении отчёта", resp.StatusCode)
		}

		// Паузу отсчитываем от полученного ответа, а не от начала запроса:
		// иначе долгий запрос съедает интервал и следующий опрос уходит сразу.
		timer := time.NewTimer(c.pollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, fmt.Errorf("partner: отчёт не готов, ожидание прервано: %w", ctx.Err())
		case <-timer.C:
		}
	}
}

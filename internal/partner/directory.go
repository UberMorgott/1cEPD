package partner

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// subscriberPage — сколько абонентов 1С отдаёт за страницу (docs/API.md §7).
const subscriberPage = 300

// Subscriber — абонент партнёра из /rest/public/subscriber.
type Subscriber struct {
	Code          string
	Name          string
	Subjects      []string
	RegNumbers    []string
	Organizations []Organization
}

// Organization — организация абонента: название и реквизиты.
type Organization struct {
	Name string
	INN  string
	KPP  string
}

// Subscribers выгружает всю базу абонентов партнёра постранично.
// См. docs/API.md §7 «Список абонентов».
func (c *Client) Subscribers(ctx context.Context) ([]Subscriber, error) {
	var result []Subscriber
	for page := 0; ; page++ {
		body, err := json.Marshal(map[string]int{"page": page, "size": subscriberPage})
		if err != nil {
			return nil, fmt.Errorf("partner: не собрать тело запроса: %w", err)
		}
		resp, err := c.post(ctx, "/rest/public/subscriber", body)
		if err != nil {
			return nil, fmt.Errorf("partner: не получить список абонентов: %w", err)
		}
		list, total, err := ParseSubscribers(resp.Body)
		if err != nil {
			return nil, err
		}
		result = append(result, list...)
		// Пустая страница — страховка от бесконечного цикла при кривом total.
		if len(list) == 0 || len(result) >= total {
			return result, nil
		}
	}
}

// ParseSubscribers разбирает страницу списка абонентов; total — всего записей.
func ParseSubscribers(raw []byte) ([]Subscriber, int, error) {
	var payload struct {
		Subscribers []struct {
			Code          string            `json:"code"`
			Name          string            `json:"name"`
			Subjects      []string          `json:"subjects"`
			RegNumbers    []json.RawMessage `json:"regNumbers"`
			Organizations []struct {
				Name string `json:"name"`
				INN  string `json:"inn"`
				KPP  string `json:"kpp"`
			} `json:"organizations"`
		} `json:"subscribers"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, 0, fmt.Errorf("partner: не разобрать список абонентов: %w", err)
	}

	list := make([]Subscriber, 0, len(payload.Subscribers))
	for _, item := range payload.Subscribers {
		subscriber := Subscriber{Code: strings.TrimSpace(item.Code), Name: strings.TrimSpace(item.Name),
			Subjects: item.Subjects}
		for _, number := range item.RegNumbers {
			if value := rawNumber(number); value != "" {
				subscriber.RegNumbers = append(subscriber.RegNumbers, value)
			}
		}
		for _, org := range item.Organizations {
			subscriber.Organizations = append(subscriber.Organizations, Organization{
				Name: strings.TrimSpace(org.Name), INN: strings.TrimSpace(org.INN), KPP: strings.TrimSpace(org.KPP),
			})
		}
		list = append(list, subscriber)
	}
	return list, payload.Total, nil
}

// Nomenclature — продукт, к которому относится регистрационный номер.
type Nomenclature struct {
	RegNumber    string
	SerialNumber string
	Name         string
}

// NomenclatureByRegNumbers узнаёт продукт по регистрационным номерам.
// Единственный метод с голым массивом в теле (docs/API.md §7).
func (c *Client) NomenclatureByRegNumbers(ctx context.Context, regNumbers []string) ([]Nomenclature, error) {
	numbers, err := parseRegNumbers(regNumbers)
	if err != nil {
		return nil, err
	}
	var result []Nomenclature
	for batch := range slices.Chunk(numbers, checkBatch) {
		body, err := json.Marshal(batch)
		if err != nil {
			return nil, fmt.Errorf("partner: не собрать тело запроса: %w", err)
		}
		resp, err := c.post(ctx, "/nomenclature/getByRegNumbers", body)
		if err != nil {
			return nil, fmt.Errorf("partner: не получить номенклатуру по регномерам: %w", err)
		}
		list, err := ParseNomenclature(resp.Body)
		if err != nil {
			return nil, err
		}
		result = append(result, list...)
	}
	return result, nil
}

// ParseNomenclature разбирает ответ nomenclature/getByRegNumbers.
func ParseNomenclature(raw []byte) ([]Nomenclature, error) {
	var items []struct {
		RegNum       json.RawMessage `json:"regNum"`
		SerialNumber string          `json:"serialNumber"`
		Name         string          `json:"name"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("partner: не разобрать номенклатуру: %w", err)
	}
	list := make([]Nomenclature, 0, len(items))
	for _, item := range items {
		list = append(list, Nomenclature{RegNumber: rawNumber(item.RegNum),
			SerialNumber: strings.TrimSpace(item.SerialNumber), Name: strings.TrimSpace(item.Name)})
	}
	return list, nil
}

// CheckITSByRegNumbers проверяет договоры 1С:ИТС по регистрационным номерам,
// пачками по 100 (docs/API.md §5).
func (c *Client) CheckITSByRegNumbers(ctx context.Context, regNumbers []string) ([]ITSCheck, error) {
	numbers, err := parseRegNumbers(regNumbers)
	if err != nil {
		return nil, err
	}
	var result []ITSCheck
	for batch := range slices.Chunk(numbers, checkBatch) {
		body, err := json.Marshal(map[string][]int64{"regNumberList": batch})
		if err != nil {
			return nil, fmt.Errorf("partner: не собрать тело запроса: %w", err)
		}
		resp, err := c.post(ctx, "/rest/public/subscription/checkItsByRegNum", body)
		if err != nil {
			return nil, fmt.Errorf("partner: не проверить договоры 1С:ИТС по регномерам: %w", err)
		}
		checks, err := ParseITSChecks(resp.Body)
		if err != nil {
			return nil, err
		}
		result = append(result, checks...)
	}
	return result, nil
}

// UserExists сообщает, есть ли на Портале ИТС пользователь с этим логином
// и этим e-mail одновременно. Оба поля обязательны (docs/API.md §7).
func (c *Client) UserExists(ctx context.Context, login, email string) (bool, error) {
	body, err := json.Marshal(map[string]string{
		"login": strings.TrimSpace(login), "email": strings.TrimSpace(email),
	})
	if err != nil {
		return false, fmt.Errorf("partner: не собрать тело запроса: %w", err)
	}
	resp, err := c.post(ctx, "/users", body)
	if err != nil {
		return false, fmt.Errorf("partner: не проверить пользователя: %w", err)
	}
	var payload struct {
		Found bool `json:"found"`
	}
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		return false, fmt.Errorf("partner: не разобрать ответ о пользователе: %w", err)
	}
	return payload.Found, nil
}

// parseRegNumbers переводит регномера в числа: в запросах 1С ждёт числа.
func parseRegNumbers(values []string) ([]int64, error) {
	numbers := make([]int64, 0, len(values))
	for _, value := range values {
		number, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("partner: регистрационный номер %q не число: %w", value, err)
		}
		numbers = append(numbers, number)
	}
	return numbers, nil
}

// rawNumber снимает кавычки с номера: 1С присылает его то числом, то строкой
// (docs/API.md §12, грабля 3). null и пустота дают пустую строку.
func rawNumber(raw json.RawMessage) string {
	value := strings.Trim(strings.TrimSpace(string(raw)), `"`)
	if value == "null" {
		return ""
	}
	return strings.TrimSpace(value)
}

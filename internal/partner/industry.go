package partner

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
)

// IndustryStatusNeeded — код «есть конфигурация, которой нужен 1С:ИТС
// Отраслевой» (107 NotEmptyList, docs/API.md §6).
const IndustryStatusNeeded = 107

// IndustryCheck — ответ проверки отраслевого сопровождения по коду абонента.
type IndustryCheck struct {
	SubscriberCode string
	// Code сравнивают по числу: у 109 статус пишется то IsNotClient, то IsNotCLIENT.
	Code        int
	Status      string
	Description string
	Programs    []IndustryProgram
}

// IndustryProgram — конфигурация, которой нужен ИТС Отраслевой.
type IndustryProgram struct {
	UIN  string
	Name string
	// Subscriptions — оформленные ИТС Отраслевые; пусто — не оформлен.
	Subscriptions []IndustrySubscription
}

// IndustrySubscription — элемент industrySubscriptionInfoList.
type IndustrySubscription struct {
	NomenclatureName string
	SerialNumber     string
	TypeDescription  string
	Begin            time.Time
	End              time.Time
}

// MissingIndustry возвращает конфигурации, которым нужен ИТС Отраслевой,
// а он не оформлен. Пусто, если нужды нет или всё оформлено.
func (c IndustryCheck) MissingIndustry() []IndustryProgram {
	if c.Code != IndustryStatusNeeded {
		return nil
	}
	var missing []IndustryProgram
	for _, program := range c.Programs {
		if len(program.Subscriptions) == 0 {
			missing = append(missing, program)
		}
	}
	return missing
}

// CheckIndustryBySubscriberCodes проверяет отраслевое сопровождение по кодам
// абонентов пачками по 100. Путь несимметричен остальным: checkIndustryBy…
// (docs/API.md §6).
func (c *Client) CheckIndustryBySubscriberCodes(ctx context.Context, codes []string) ([]IndustryCheck, error) {
	var result []IndustryCheck
	for batch := range slices.Chunk(codes, checkBatch) {
		body, err := json.Marshal(map[string][]string{"subscriberCodeList": batch})
		if err != nil {
			return nil, fmt.Errorf("partner: не собрать тело запроса: %w", err)
		}
		resp, err := c.post(ctx, "/rest/public/industry/checkIndustryBySubscriberCode", body)
		if err != nil {
			return nil, fmt.Errorf("partner: не проверить ИТС Отраслевой: %w", err)
		}
		checks, err := ParseIndustryChecks(resp.Body)
		if err != nil {
			return nil, err
		}
		result = append(result, checks...)
	}
	return result, nil
}

// ParseIndustryChecks разбирает ответ checkIndustryBySubscriberCode.
// programInfoList и industrySubscriptionInfoList приходят и null, и списком.
func ParseIndustryChecks(raw []byte) ([]IndustryCheck, error) {
	var items []struct {
		Status         string `json:"status"`
		Code           int    `json:"code"`
		Description    string `json:"description"`
		SubscriberCode string `json:"subscriberCode"`
		Programs       []struct {
			UIN           string `json:"uin"`
			Name          string `json:"name"`
			Subscriptions []struct {
				NomenclatureName string `json:"nomenclatureName"`
				SerialNumber     string `json:"serialNumber"`
				Type             struct {
					Description string `json:"description"`
				} `json:"industrySubscriptionTypeInfo"`
				BeginDate *time.Time `json:"beginDate"`
				EndDate   *time.Time `json:"endDate"`
			} `json:"industrySubscriptionInfoList"`
		} `json:"programInfoList"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("partner: не разобрать ответ проверки ИТС Отраслевого: %w", err)
	}

	list := make([]IndustryCheck, 0, len(items))
	for _, item := range items {
		check := IndustryCheck{
			SubscriberCode: strings.TrimSpace(item.SubscriberCode), Code: item.Code,
			Status: item.Status, Description: strings.TrimSpace(item.Description),
		}
		for _, p := range item.Programs {
			program := IndustryProgram{UIN: p.UIN, Name: strings.TrimSpace(p.Name)}
			for _, s := range p.Subscriptions {
				sub := IndustrySubscription{
					NomenclatureName: strings.TrimSpace(s.NomenclatureName),
					SerialNumber:     strings.TrimSpace(s.SerialNumber),
					TypeDescription:  strings.TrimSpace(s.Type.Description),
				}
				if s.BeginDate != nil {
					sub.Begin = s.BeginDate.UTC()
				}
				if s.EndDate != nil {
					sub.End = s.EndDate.UTC()
				}
				program.Subscriptions = append(program.Subscriptions, sub)
			}
			check.Programs = append(check.Programs, program)
		}
		list = append(list, check)
	}
	return list, nil
}

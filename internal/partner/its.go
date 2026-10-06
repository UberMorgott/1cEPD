package partner

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
)

// checkBatch — сколько ключей 1С принимает в одной проверке (docs/API.md §4).
const checkBatch = 100

// ITSStatusSuccess — код ответа «договор 1С:ИТС оформлен» (docs/API.md §5).
const ITSStatusSuccess = 1

// ITSCheck — ответ проверки договора 1С:ИТС по одному коду абонента.
type ITSCheck struct {
	SubscriberCode string
	// RegNumber — ключ ответа checkItsByRegNum; пусто у проверки по коду абонента.
	RegNumber string
	// Code — числовой код статуса. Сравнивать надо по нему: написание Status
	// у одного и того же кода в разных методах разное (docs/API.md §12, грабля 9).
	Code        int
	Status      string
	Description string
	Contracts   []ITSContract
}

// ITSContract — договор из itsContractInfo.
type ITSContract struct {
	Description string
	Start       time.Time
	End         time.Time
	TypeUIN     string
	TypeName    string
	// TypeNameForUser — название вида для людей: «1С:Комплект поддержки ПРОФ».
	TypeNameForUser string
	// TypeNumber — publicSubscriptionTypeNumber; nil, если 1С его не прислала.
	TypeNumber *int
}

// CheckITSBySubscriberCodes проверяет договоры 1С:ИТС по кодам абонентов,
// пачками по 100. См. docs/API.md §5 и §11, рецепт «у кого кончается договор».
func (c *Client) CheckITSBySubscriberCodes(ctx context.Context, codes []string) ([]ITSCheck, error) {
	var result []ITSCheck
	for batch := range slices.Chunk(codes, checkBatch) {
		body, err := json.Marshal(map[string][]string{"subscriberCodeList": batch})
		if err != nil {
			return nil, fmt.Errorf("partner: не собрать тело запроса: %w", err)
		}
		resp, err := c.post(ctx, "/rest/public/subscription/checkItsBySubscriberCode", body)
		if err != nil {
			return nil, fmt.Errorf("partner: не проверить договоры 1С:ИТС: %w", err)
		}
		checks, err := ParseITSChecks(resp.Body)
		if err != nil {
			return nil, err
		}
		result = append(result, checks...)
	}
	return result, nil
}

// ParseITSChecks разбирает ответ checkItsBySubscriberCode.
func ParseITSChecks(raw []byte) ([]ITSCheck, error) {
	var items []struct {
		Status         string `json:"status"`
		Code           int    `json:"code"`
		Description    string `json:"description"`
		SubscriberCode string `json:"subscriberCode"`
		// RegNumber в ответе бывает строкой, а в запросе — число (docs/API.md §5).
		RegNumber json.RawMessage `json:"regNumber"`
		Contracts []struct {
			Description string     `json:"description"`
			StartDate   *time.Time `json:"startDate"`
			EndDate     *time.Time `json:"endDate"`
			Type        struct {
				UIN         string `json:"uin"`
				Name        string `json:"name"`
				NameForUser string `json:"nameForUser"`
				Number      *int   `json:"publicSubscriptionTypeNumber"`
			} `json:"itsContractType"`
		} `json:"itsContractInfo"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("partner: не разобрать ответ проверки договоров 1С:ИТС: %w", err)
	}

	list := make([]ITSCheck, 0, len(items))
	for _, item := range items {
		check := ITSCheck{
			SubscriberCode: strings.TrimSpace(item.SubscriberCode),
			RegNumber:      rawNumber(item.RegNumber),
			Code:           item.Code, Status: item.Status,
			Description: strings.TrimSpace(item.Description),
		}
		for _, contract := range item.Contracts {
			parsed := ITSContract{
				Description: strings.TrimSpace(contract.Description),
				TypeUIN:     contract.Type.UIN, TypeName: strings.TrimSpace(contract.Type.Name),
				TypeNameForUser: strings.TrimSpace(contract.Type.NameForUser),
				TypeNumber:      contract.Type.Number,
			}
			if contract.StartDate != nil {
				parsed.Start = contract.StartDate.UTC()
			}
			if contract.EndDate != nil {
				parsed.End = contract.EndDate.UTC()
			}
			check.Contracts = append(check.Contracts, parsed)
		}
		list = append(list, check)
	}
	return list, nil
}

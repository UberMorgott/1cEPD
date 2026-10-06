package partner

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// OptionReportTypes — виды отчёта по опциям (docs/API.md §8.3) в порядке
// важности: суточная задача строит их по одному, и первыми идут те, о которых
// спрашивают чаще.
var OptionReportTypes = []string{
	"REPORTING", "SIGN", "CLOUD_BACKUP", "COUNTERAGENT", "SPARK_RISKS", "LINK",
	"NOMENCLATURE", "ESS", "DOCUMENT_RECOGNITION", "CHECK_SCAN", "MAG1C",
}

// Состояния отчёта по опциям.
const (
	OptionReportProcessing = "PROCESSING"
	OptionReportOK         = "OK"
	OptionReportError      = "ERROR"
)

// OptionReport — ответ GET /rest/public/option/billing-report/{reportUeid}.
type OptionReport struct {
	State     string
	Completed time.Time
	Entries   []OptionEntry
}

// OptionEntry — тарифы одного абонента.
type OptionEntry struct {
	SubscriberCode string
	Tariffs        []OptionTariff
}

// OptionTariff — тариф с опциями сервиса.
type OptionTariff struct {
	Name string
	// TypeNumber — publicSubscriptionTypeNumber; nil, если 1С прислала null.
	TypeNumber *int
	BuyDate    time.Time
	OrgName    string
	OrgINN     string
	OrgKPP     string
	Start      time.Time
	End        time.Time
	Options    []Option
}

// Option — опция тарифа. У качественной (Quantitative false) объёмов нет.
type Option struct {
	Name         string
	Quantitative bool
	MaxVolume    *float64
	UsedVolume   *float64
}

// OptionBillingReport строит отчёт по опциям одного вида по всем абонентам
// партнёра и ждёт результат. Повторно ставить задачу при ошибке нельзя —
// это тратит часовой лимит 1С.
func (c *Client) OptionBillingReport(ctx context.Context, reportType string) (OptionReport, error) {
	body, err := json.Marshal(map[string]string{"type": reportType})
	if err != nil {
		return OptionReport{}, fmt.Errorf("partner: не собрать тело запроса: %w", err)
	}
	resp, err := c.post(ctx, "/rest/public/option/billing-report", body)
	if err != nil {
		return OptionReport{}, fmt.Errorf("partner: не заказать отчёт по опциям %s: %w", reportType, err)
	}
	var task struct {
		ReportUeid string `json:"reportUeid"`
		Error      string `json:"error"`
	}
	if err := json.Unmarshal(resp.Body, &task); err != nil {
		return OptionReport{}, fmt.Errorf("partner: не разобрать ответ на заказ отчёта по опциям: %w", err)
	}
	if task.Error != "" {
		return OptionReport{}, fmt.Errorf("partner: 1С отказала в отчёте по опциям %s: %s", reportType, task.Error)
	}
	if task.ReportUeid == "" {
		return OptionReport{}, fmt.Errorf("partner: 1С не вернула идентификатор отчёта по опциям")
	}

	path := "/rest/public/option/billing-report/" + task.ReportUeid
	for {
		resp, err := c.get(ctx, path)
		if err != nil {
			return OptionReport{}, fmt.Errorf("partner: не забрать отчёт по опциям: %w", err)
		}
		if resp.StatusCode == http.StatusOK && len(resp.Body) > 0 {
			report, err := ParseOptionReport(resp.Body)
			if err != nil {
				return OptionReport{}, err
			}
			switch report.State {
			case OptionReportOK:
				return report, nil
			case OptionReportError:
				return OptionReport{}, fmt.Errorf("partner: 1С не построила отчёт по опциям %s", reportType)
			}
		}

		timer := time.NewTimer(c.pollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return OptionReport{}, fmt.Errorf("partner: отчёт по опциям не готов, ожидание прервано: %w", ctx.Err())
		case <-timer.C:
		}
	}
}

// ParseOptionReport разбирает ответ отчёта по опциям. report при PROCESSING и
// ERROR отсутствует, tariffs и options бывают пустыми.
func ParseOptionReport(raw []byte) (OptionReport, error) {
	var payload struct {
		State         string     `json:"state"`
		CompletedDate *time.Time `json:"completedDate"`
		Report        *struct {
			Entries []struct {
				SubscriberCode string `json:"subscriberCode"`
				Tariffs        []struct {
					Name       string     `json:"name"`
					TypeNumber *int       `json:"publicSubscriptionTypeNumber"`
					BuyDate    *time.Time `json:"buyDate"`
					OrgName    string     `json:"userOrganizationName"`
					OrgINN     string     `json:"userOrganizationInn"`
					OrgKPP     string     `json:"userOrganizationKpp"`
					StartDate  *time.Time `json:"startDate"`
					EndDate    *time.Time `json:"endDate"`
					Options    []struct {
						Name         string   `json:"name"`
						Quantitative bool     `json:"quantitative"`
						MaxVolume    *float64 `json:"maxVolume"`
						UsedVolume   *float64 `json:"usedVolume"`
					} `json:"options"`
				} `json:"tariffs"`
			} `json:"entries"`
		} `json:"report"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return OptionReport{}, fmt.Errorf("partner: не разобрать отчёт по опциям: %w", err)
	}

	report := OptionReport{State: payload.State, Completed: timeOrZero(payload.CompletedDate)}
	if payload.Report == nil {
		return report, nil
	}
	for _, e := range payload.Report.Entries {
		entry := OptionEntry{SubscriberCode: strings.TrimSpace(e.SubscriberCode)}
		for _, t := range e.Tariffs {
			tariff := OptionTariff{
				Name: strings.TrimSpace(t.Name), TypeNumber: t.TypeNumber, BuyDate: timeOrZero(t.BuyDate),
				OrgName: strings.TrimSpace(t.OrgName), OrgINN: strings.TrimSpace(t.OrgINN),
				OrgKPP: strings.TrimSpace(t.OrgKPP), Start: timeOrZero(t.StartDate), End: timeOrZero(t.EndDate),
			}
			for _, o := range t.Options {
				tariff.Options = append(tariff.Options, Option{
					Name: strings.TrimSpace(o.Name), Quantitative: o.Quantitative,
					MaxVolume: o.MaxVolume, UsedVolume: o.UsedVolume,
				})
			}
			entry.Tariffs = append(entry.Tariffs, tariff)
		}
		report.Entries = append(report.Entries, entry)
	}
	return report, nil
}

func timeOrZero(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return t.UTC()
}

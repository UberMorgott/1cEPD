package service

import (
	"maps"

	"partnerops/internal/partner"
)

// IdentifierTraffic — сведения об идентификаторе из отчёта трафика ЭДО: их нет
// в биллинге. Счётчики — документы за период отчёта.
type IdentifierTraffic struct {
	PeriodFrom     string `json:"periodFrom"`
	PeriodTo       string `json:"periodTo"`
	Operator       string `json:"operator"`
	LinkCreated    string `json:"linkCreated"`
	IDRegistered   string `json:"idRegistered"`
	SupportFrom    string `json:"supportFrom"`
	SupportTo      string `json:"supportTo"`
	InvoicesIn     int64  `json:"invoicesIn"`
	NonInvoicesIn  int64  `json:"nonInvoicesIn"`
	EPDIn          int64  `json:"epdIn"`
	InvoicesOut    int64  `json:"invoicesOut"`
	NonInvoicesOut int64  `json:"nonInvoicesOut"`
	EPDOut         int64  `json:"epdOut"`
}

// TrafficReport — строки одного отчёта трафика и его период ГГГГ-ММ.
type TrafficReport struct {
	PeriodFrom string
	PeriodTo   string
	Rows       []partner.EDOTrafficRow
}

// TrafficByID сводит отчёты трафика по идентификаторам. Отчёты идут по
// убыванию охвата (год, затем текущий месяц): идентификатор берётся из первого,
// где он есть, — так у нового клиента видны хотя бы данные текущего месяца.
// Строки одного идентификатора внутри отчёта складываются.
func TrafficByID(reports ...TrafficReport) map[string]IdentifierTraffic {
	result := map[string]IdentifierTraffic{}
	for _, report := range reports {
		own := map[string]IdentifierTraffic{}
		for _, r := range report.Rows {
			if r.EDOID == "" {
				continue
			}
			if _, done := result[r.EDOID]; done {
				continue
			}
			t, ok := own[r.EDOID]
			if !ok {
				t = IdentifierTraffic{PeriodFrom: report.PeriodFrom, PeriodTo: report.PeriodTo}
			}
			t.Operator = firstNonEmpty(t.Operator, r.Operator)
			t.LinkCreated = firstNonEmpty(t.LinkCreated, r.LinkCreated)
			t.IDRegistered = firstNonEmpty(t.IDRegistered, r.IDRegistered)
			t.SupportFrom = firstNonEmpty(t.SupportFrom, r.SupportFrom)
			t.SupportTo = firstNonEmpty(t.SupportTo, r.SupportTo)
			t.InvoicesIn += r.InvoicesIn
			t.NonInvoicesIn += r.NonInvoicesIn
			t.EPDIn += r.EPDIn
			t.InvoicesOut += r.InvoicesOut
			t.NonInvoicesOut += r.NonInvoicesOut
			t.EPDOut += r.EPDOut
			own[r.EDOID] = t
		}
		maps.Copy(result, own)
	}
	return result
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

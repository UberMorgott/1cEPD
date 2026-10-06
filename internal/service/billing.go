package service

import (
	"sort"

	"partnerops/internal/money"
	"partnerops/internal/partner"
)

// lowRemainderShare — доля лимита, ниже которой остаток считается низким.
const lowRemainderShare = 0.1

// BillingClient — строка отчёта по клиенту.
type BillingClient struct {
	EDOID        string       `json:"edoId"`
	ClientName   string       `json:"clientName"`
	INN          string       `json:"inn"`
	KPP          string       `json:"kpp"`
	Login        string       `json:"login"`
	Owner        string       `json:"owner"`
	Tariffs      string       `json:"tariffs"`
	Limit        *int64       `json:"limit"`
	Used         int64        `json:"used"`
	Remaining    *int64       `json:"remaining"`
	Billable     int64        `json:"billable"`
	ClientAmount money.Amount `json:"-"`
	AmountText   string       `json:"amount"`
	// PartnerAmountText — доход партнёра по этой строке. Приходит из отчёта 1С
	// отдельной колонкой и отличается от суммы, выставляемой клиенту.
	PartnerAmount     money.Amount `json:"-"`
	PartnerAmountText string       `json:"partnerAmount"`
	OverLimit         bool         `json:"overLimit"`
	LowRemainder      bool         `json:"lowRemainder"`
}

// BillingReport — то, что показывается на экране биллинга.
type BillingReport struct {
	Clients     []BillingClient `json:"clients"`
	NeedsAction []BillingClient `json:"needsAction"`
	TotalDue    money.Amount    `json:"-"`
	TotalText   string          `json:"totalDue"`
}

// BuildBilling считает остатки квот и собирает список тех, кем надо заняться.
//
// Складывается только «Сумма для клиента»: колонка «по владельцу» агрегатная
// и повторяется в строках одного владельца, суммирование дало бы двойной счёт.
func BuildBilling(rows []partner.EDOBillingRow) BillingReport {
	report := BillingReport{Clients: make([]BillingClient, 0, len(rows))}

	for _, r := range rows {
		client := BillingClient{
			EDOID: r.EDOID, ClientName: r.ClientName, INN: r.INN, KPP: r.KPP,
			Login: r.Login, Owner: r.Owner, Tariffs: r.ITSTariffs,
			Limit: r.Limit, Used: r.Packets, Billable: r.PacketsBillable,
			ClientAmount: r.ClientAmount, AmountText: r.ClientAmount.String(),
			PartnerAmount: r.PartnerAmount, PartnerAmountText: r.PartnerAmount.String(),
			OverLimit: r.PacketsBillable > 0,
		}

		if r.Limit != nil {
			remaining := *r.Limit - r.Packets
			client.Remaining = &remaining
			client.LowRemainder = *r.Limit > 0 &&
				float64(remaining) <= float64(*r.Limit)*lowRemainderShare
		}

		report.TotalDue += r.ClientAmount
		report.Clients = append(report.Clients, client)

		if client.OverLimit || client.LowRemainder {
			report.NeedsAction = append(report.NeedsAction, client)
		}
	}

	// Сначала те, у кого больше сумма к выставлению, затем по остатку.
	sort.SliceStable(report.NeedsAction, func(i, j int) bool {
		return report.NeedsAction[i].ClientAmount > report.NeedsAction[j].ClientAmount
	})

	report.TotalText = report.TotalDue.String()
	return report
}

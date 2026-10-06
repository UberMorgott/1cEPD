package partner

import "partnerops/internal/money"

// EDOBillingRow — строка отчёта биллинга ЭДО.
// Названия полей соответствуют колонкам CSV, см. docs/API.md §8.1.
type EDOBillingRow struct {
	PartnerCode     string
	PartnerName     string
	Owner           string // код абонента-владельца, пустой означает потерянную связь
	Login           string
	EDOID           string
	ClientName      string
	INN             string
	KPP             string
	ITSTariffs      string
	Limit           *int64 // nil, если лимит не задан
	InvoicesOut     int64
	NonInvoicesOut  int64
	Packets         int64
	PacketsByOwner  int64
	Discount        int64
	PacketsBillable int64
	TariffAmount    money.Amount
	ClientAmount    money.Amount
	PartnerAmount   money.Amount
	Extra           string
}

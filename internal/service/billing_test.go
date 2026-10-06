package service

import (
	"testing"

	"partnerops/internal/money"
	"partnerops/internal/partner"
)

func TestBillingComputesRemainder(t *testing.T) {
	limit := int64(100)
	rows := []partner.EDOBillingRow{
		{EDOID: "2AE-A", ClientName: "ООО Тест", INN: "7700000001",
			Limit: &limit, Packets: 27, PacketsBillable: 0},
	}

	report := BuildBilling(rows)

	if len(report.Clients) != 1 {
		t.Fatalf("клиентов %d, ожидали 1", len(report.Clients))
	}
	client := report.Clients[0]
	if client.Limit == nil || *client.Limit != 100 {
		t.Errorf("лимит не сохранён: %v", client.Limit)
	}
	if client.Remaining == nil || *client.Remaining != 73 {
		t.Errorf("остаток = %v, ожидали 73", client.Remaining)
	}
	if client.OverLimit {
		t.Error("клиент в пределах лимита не должен быть отмечен как превысивший")
	}
}

func TestBillingFlagsOverLimit(t *testing.T) {
	rows := []partner.EDOBillingRow{
		{EDOID: "2AE-B", ClientName: "ООО Вторма", INN: "4200000320",
			Packets: 71, PacketsBillable: 71, ClientAmount: money.Amount(710000)},
	}

	report := BuildBilling(rows)

	if len(report.NeedsAction) != 1 {
		t.Fatalf("в «требует действия» %d записей, ожидали 1", len(report.NeedsAction))
	}
	if !report.Clients[0].OverLimit {
		t.Error("клиент с пакетами к оплате должен быть отмечен")
	}
	if report.TotalDue != money.Amount(710000) {
		t.Errorf("сумма к выставлению = %d, ожидали 710000", report.TotalDue)
	}
}

func TestBillingWarnsOnLowRemainder(t *testing.T) {
	limit := int64(50)
	rows := []partner.EDOBillingRow{
		{EDOID: "2AE-C", ClientName: "ООО Почти", Limit: &limit, Packets: 47},
	}

	report := BuildBilling(rows)

	if !report.Clients[0].LowRemainder {
		t.Error("остаток 3 из 50 должен помечаться как низкий")
	}
	if len(report.NeedsAction) != 1 {
		t.Errorf("в «требует действия» %d записей, ожидали 1", len(report.NeedsAction))
	}
}

func TestBillingIgnoresRowsWithoutLimit(t *testing.T) {
	rows := []partner.EDOBillingRow{
		{EDOID: "2AE-D", ClientName: "ООО Без лимита", Packets: 5},
	}

	report := BuildBilling(rows)

	if report.Clients[0].Remaining != nil {
		t.Error("без лимита остаток посчитать нельзя, ожидали nil")
	}
	if report.Clients[0].LowRemainder {
		t.Error("без лимита предупреждать не о чем")
	}
}

func TestBillingDoesNotSumAggregateColumn(t *testing.T) {
	// «Сумма пакетов документов ЭДО по владельцу» относится к владельцу и повторяется
	// в его строках. Складывать её построчно нельзя — получится двойной счёт.
	rows := []partner.EDOBillingRow{
		{EDOID: "2AE-E", ClientName: "Клиент", Owner: "CL-1 - Иванов",
			Packets: 5, PacketsByOwner: 10, PacketsBillable: 5, ClientAmount: money.Amount(50000)},
		{EDOID: "2AE-F", ClientName: "Клиент", Owner: "CL-1 - Иванов",
			Packets: 5, PacketsByOwner: 10, PacketsBillable: 0},
	}

	report := BuildBilling(rows)

	if report.TotalDue != money.Amount(50000) {
		t.Errorf("сумма = %d, ожидали 50000: агрегатную колонку складывать нельзя", report.TotalDue)
	}
}

func TestBillingExposesPartnerAmount(t *testing.T) {
	// Доход партнёра приходит из 1С отдельной колонкой и не равен сумме клиенту.
	rows := []partner.EDOBillingRow{
		{EDOID: "2AE-P", ClientName: "ООО Тест",
			ClientAmount: money.Amount(710000), PartnerAmount: money.Amount(355000)},
	}

	report := BuildBilling(rows)

	if got := report.Clients[0].PartnerAmountText; got != "355,00" {
		t.Errorf("доход партнёра = %q, ожидали 355,00", got)
	}
	if got := report.Clients[0].AmountText; got != "710,00" {
		t.Errorf("счёт клиенту = %q, ожидали 710,00", got)
	}
}

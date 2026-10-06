package service

import (
	"testing"

	"partnerops/internal/partner"
)

func TestTrafficByIDPrefersYearAndSumsRows(t *testing.T) {
	year := TrafficReport{PeriodFrom: "2025-09", PeriodTo: "2026-08", Rows: []partner.EDOTrafficRow{
		{EDOID: "2AE-1", Operator: "Такском", LinkCreated: "2017-02-15", InvoicesOut: 10, EPDIn: 1},
		{EDOID: "2AE-1", IDRegistered: "2020-05-21", InvoicesOut: 5},
	}}
	month := TrafficReport{PeriodFrom: "2026-09", PeriodTo: "2026-09", Rows: []partner.EDOTrafficRow{
		{EDOID: "2AE-1", Operator: "Другой", InvoicesOut: 99},
		{EDOID: "2AE-NEW", Operator: "Калуга Астрал", LinkCreated: "2026-09-20", InvoicesOut: 2},
	}}
	got := TrafficByID(year, month)

	old := got["2AE-1"]
	if old.Operator != "Такском" || old.IDRegistered != "2020-05-21" || old.InvoicesOut != 15 ||
		old.EPDIn != 1 || old.PeriodFrom != "2025-09" {
		t.Errorf("идентификатор из годового отчёта %+v", old)
	}
	if fresh := got["2AE-NEW"]; fresh.PeriodFrom != "2026-09" || fresh.LinkCreated != "2026-09-20" {
		t.Errorf("новый идентификатор %+v", fresh)
	}
}

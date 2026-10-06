package partner

import (
	"strings"
	"testing"
)

// Заголовок живого отчёта трафика, строки обезличены.
const trafficHeader = "\xef\xbb\xbfНазвание организации;ИНН;КПП;Абонент;Оператор;Дата сопровождения с;" +
	"Дата сопровождения по;Идентификатор ЭДО;Логин;Дата создания связи;" +
	"Дата регистрации идентификатора ЭДО;СФ (вх.);не_СФ (вх.);ЭПД (вх.);СФ (исх.);не_СФ (исх.);ЭПД (исх.)\n"

func TestParseEDOTrafficReadsAllColumns(t *testing.T) {
	raw := trafficHeader +
		"ООО Альфа;7700000001;770001001;CL-1 - a;Такском;01.09.2026;31.08.2027;2AE-1;a@example.ru;" +
		"15.02.2017;21.05.2020 0:00:00;1;2;3;4;5;6\n" +
		"ИП Бета;770000000200;;FR-FR-2 - b;Калуга Астрал;;;2AE-2;b;30.09.2019;30.09.2019;0;0;0;0;0;0\n"
	rows, err := ParseEDOTraffic([]byte(raw))
	if err != nil {
		t.Fatalf("ParseEDOTraffic: %v", err)
	}
	want := EDOTrafficRow{
		ClientName: "ООО Альфа", INN: "7700000001", KPP: "770001001", Subscriber: "CL-1 - a",
		EDOID: "2AE-1", Login: "a@example.ru", Operator: "Такском",
		SupportFrom: "2026-09-01", SupportTo: "2027-08-31",
		LinkCreated: "2017-02-15", IDRegistered: "2020-05-21",
		InvoicesIn: 1, NonInvoicesIn: 2, EPDIn: 3, InvoicesOut: 4, NonInvoicesOut: 5, EPDOut: 6,
	}
	if len(rows) != 2 || rows[0] != want {
		t.Fatalf("строка %+v", rows[0])
	}
	if rows[1].SupportFrom != "" || rows[1].Operator != "Калуга Астрал" {
		t.Errorf("вторая строка %+v", rows[1])
	}
}

func TestParseEDOTrafficRejectsBadDate(t *testing.T) {
	raw := trafficHeader + "ООО;1;;CL-1;Оп;;;2AE-1;a;2017-02-15;;0;0;0;0;0;0\n"
	if _, err := ParseEDOTraffic([]byte(raw)); err == nil || !strings.Contains(err.Error(), "Дата создания связи") {
		t.Errorf("ошибка %v", err)
	}
}

package partner

import (
	"strings"
	"testing"
)

// Выгрузка «Детализация биллинга» ЭПД: заголовок портала, данные вымышленные.
const epdBillingHeader = "Код партнера;Название партнера;Владелец;Логин;Ид_ЭДО клиента;Наименование клиента;" +
	"ИНН клиента;КПП клиента;Тарифы ИТС;Лимит;Кол-во документов ЭПД;Сумма документов ЭПД по владельцу;Льгота;" +
	"Количество документов ЭПД к оплате;Тариф для клиента;Сумма для клиента;Сумма для партнера;Исх. ЭТРН;Исх. ЭПЛ;" +
	"Дополнительная информация"

func epdBillingSample() string {
	return "\ufeff" + epdBillingHeader + "\r\n" +
		"10001;Партнёр Тест;FR-FR-900001 - owner@example.test;1c-fresh_owner@example.test;2AE-T1;" +
		"Индивидуальный предприниматель Тестов Тест Тестович;770000000101;;ИТСааС тест(1): подп. Фреш: 1;0;0;32;0;32;7,00;224,00;0;;;\r\n" +
		// Пустой владелец — тот же, что строкой выше.
		"10001;Партнёр Тест;;1c-fresh_owner@example.test;2AE-T2;" +
		"\"Общество с ограниченной ответственностью \"\"Пример; и Ко\"\"\";7700000102;770001001;;;29;;;;;;;29;0;\r\n" +
		"10001;Партнёр Тест;CL-900002 - Образцова Анна Петровна;anna-login;2AE-T3;ООО Образец;7700000103;770001001;;;5;;;;;;;;;\r\n" +
		";;;;;;;;;;;;;;;;;;;\r\n"
}

func TestParseEPDBilling(t *testing.T) {
	rows, err := ParseEPDBilling([]byte(epdBillingSample()))
	if err != nil {
		t.Fatalf("ParseEPDBilling: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("строк %d, ожидали 3: %+v", len(rows), rows)
	}
	ip := rows[0]
	if ip.OwnerCode != "FR-FR-900001" || ip.OwnerContact != "owner@example.test" || ip.KPP != "" ||
		ip.INN != "770000000101" || ip.EDOID != "2AE-T1" || ip.ITSTariffs != "ИТСааС тест(1): подп. Фреш: 1" {
		t.Errorf("ИП: %+v", ip)
	}
	org := rows[1]
	if org.OwnerCode != "FR-FR-900001" || org.Owner != "FR-FR-900001 - owner@example.test" {
		t.Errorf("владелец не перенесён со строки выше: %+v", org)
	}
	if org.ClientName != `Общество с ограниченной ответственностью "Пример; и Ко"` || org.EPDDocs != 29 {
		t.Errorf("название в кавычках или счётчик: %+v", org)
	}
	if rows[2].OwnerCode != "CL-900002" || rows[2].OwnerContact != "Образцова Анна Петровна" {
		t.Errorf("новый владелец: %+v", rows[2])
	}
}

func TestParseEPDBillingWindows1251(t *testing.T) {
	sample := strings.TrimPrefix(epdBillingSample(), "\ufeff")
	// Обратная таблица из самого декодера: байт 1251 по символу.
	encode := map[rune]byte{}
	for b := byte(0); ; b++ {
		if r := []rune(string(decodeCP1251([]byte{b}))); r[0] != '\ufffd' {
			encode[r[0]] = b
		}
		if b == 0xFF {
			break
		}
	}
	raw := make([]byte, 0, len(sample))
	for _, r := range sample {
		b, ok := encode[r]
		if !ok {
			t.Fatalf("символ %q не из 1251", r)
		}
		raw = append(raw, b)
	}
	rows, err := ParseEPDBilling(raw)
	if err != nil {
		t.Fatalf("ParseEPDBilling 1251: %v", err)
	}
	if len(rows) != 3 || rows[2].OwnerContact != "Образцова Анна Петровна" {
		t.Errorf("1251 разобран неверно: %+v", rows)
	}
}

func TestParseEPDBillingRejectsOtherReports(t *testing.T) {
	if _, err := ParseEPDBilling([]byte("Код партнера;Владелец\r\n1;2\r\n")); err == nil {
		t.Error("отчёт без колонок выгрузки ЭПД принят")
	}
}

// Перенос владельца — только в выгрузке ЭПД: в биллинге ЭДО пустой владелец
// значит «не привязан» (docs/API.md §8.1).
func TestParseEDOBillingKeepsEmptyOwner(t *testing.T) {
	header := strings.Join(edoBillingColumns, ";")
	line := func(owner, id string) string {
		fields := make([]string, len(edoBillingColumns))
		fields[2], fields[4] = owner, id
		return strings.Join(fields, ";")
	}
	raw := header + "\r\n" + line("CL-1 - Тест", "2AE-A") + "\r\n" + line("", "2AE-B") + "\r\n"
	rows, err := ParseEDOBilling([]byte(raw))
	if err != nil {
		t.Fatalf("ParseEDOBilling: %v", err)
	}
	if len(rows) != 2 || rows[1].Owner != "" {
		t.Errorf("пустой владелец биллинга ЭДО изменён: %+v", rows)
	}
}

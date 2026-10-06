package partner

import (
	"os"
	"strings"
	"testing"
)

func TestParseEDOBilling(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/edo-billing.csv")
	if err != nil {
		t.Fatalf("не прочитать фикстуру: %v", err)
	}

	rows, err := ParseEDOBilling(raw)
	if err != nil {
		t.Fatalf("ParseEDOBilling вернул ошибку: %v", err)
	}
	if len(rows) != 23 {
		t.Fatalf("разобрано %d строк, ожидали 23", len(rows))
	}

	first := rows[0]
	if first.EDOID == "" {
		t.Error("EDOID пуст в первой строке")
	}
	if first.INN == "" {
		t.Error("INN пуст в первой строке")
	}
	if first.ClientName == "" {
		t.Error("ClientName пуст в первой строке")
	}

	// В фикстуре есть строки без владельца — это ожидаемое состояние, не ошибка.
	orphans := 0
	for _, r := range rows {
		if r.Owner == "" {
			orphans++
		}
	}
	if orphans != 7 {
		t.Errorf("строк без владельца %d, ожидали 7", orphans)
	}
}

func TestParseEDOBillingHandlesEmptyNumbers(t *testing.T) {
	csv := "\ufeffКод партнера;Название партнера;Владелец;Логин;Ид_ЭДО клиента;" +
		"Наименование клиента;ИНН клиента;КПП клиента;Тарифы ИТС;Лимит;СФ_исх;не_СФ_исх;" +
		"Кол-во пакетов документов ЭДО;Сумма пакетов документов ЭДО по владельцу;Льгота;" +
		"Количество пакетов документов ЭДО к оплате;Тариф для клиента;Сумма для клиента;" +
		"Сумма для партнера;Дополнительная информация\r\n" +
		"99999;Партнёр;;login;2AE-X;\"ООО \"\"Тест\"\"\";7700000000;770001001;;;0;0;0;;;;;;;\r\n"

	rows, err := ParseEDOBilling([]byte(csv))
	if err != nil {
		t.Fatalf("ParseEDOBilling вернул ошибку: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("разобрано %d строк, ожидали 1", len(rows))
	}
	if rows[0].Limit != nil {
		t.Errorf("Limit = %v, ожидали nil при пустом поле", *rows[0].Limit)
	}
	if rows[0].ClientName != `ООО "Тест"` {
		t.Errorf("ClientName = %q, ожидали ООО \"Тест\"", rows[0].ClientName)
	}
	if rows[0].ClientAmount != 0 {
		t.Errorf("ClientAmount = %d, ожидали 0", rows[0].ClientAmount)
	}
}

// edoHeader — заголовок отчёта для тестов, собранный из ожидаемых колонок.
const edoHeader = "\ufeff" + "Код партнера;Название партнера;Владелец;Логин;Ид_ЭДО клиента;" +
	"Наименование клиента;ИНН клиента;КПП клиента;Тарифы ИТС;Лимит;СФ_исх;не_СФ_исх;" +
	"Кол-во пакетов документов ЭДО;Сумма пакетов документов ЭДО по владельцу;Льгота;" +
	"Количество пакетов документов ЭДО к оплате;Тариф для клиента;Сумма для клиента;" +
	"Сумма для партнера;Дополнительная информация\r\n"

// Строка с числом полей, отличным от заголовка, — это повреждённый отчёт.
func TestParseEDOBillingRejectsShortRow(t *testing.T) {
	raw := edoHeader + "99999;Партнёр;;login;2AE-X;Клиент;7700000000;770001001\r\n"
	_, err := ParseEDOBilling([]byte(raw))
	if err == nil {
		t.Fatal("ожидали ошибку о неверном числе полей")
	}
	if !strings.Contains(err.Error(), "2") {
		t.Errorf("ошибка %q не содержит номер строки", err.Error())
	}
}

// Пустые строки в середине отчёта пропускаются как и раньше.
func TestParseEDOBillingSkipsBlankRows(t *testing.T) {
	row := "99999;Партнёр;;login;2AE-X;Клиент;7700000000;770001001;;;0;0;0;0;0;0;0;0;0;\r\n"
	blank := strings.Repeat(";", 19) + "\r\n"
	rows, err := ParseEDOBilling([]byte(edoHeader + row + blank + row))
	if err != nil {
		t.Fatalf("ParseEDOBilling вернул ошибку: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("разобрано %d строк, ожидали 2", len(rows))
	}
}

// Ненулевую дробную часть счётчика нельзя молча отбрасывать.
func TestParseCountRejectsNonZeroFraction(t *testing.T) {
	if got, err := parseCount("2,000"); err != nil || got != 2 {
		t.Errorf("parseCount(\"2,000\") = %d, %v; ожидали 2, nil", got, err)
	}
	if got, err := parseCount("2,999"); err == nil {
		t.Errorf("parseCount(\"2,999\") = %d, ожидали ошибку", got)
	}
}

// Лимит приходит и в виде "2,000" — это не повод ронять весь отчёт.
func TestParseEDOBillingAcceptsFractionalLimit(t *testing.T) {
	raw := edoHeader + "99999;Партнёр;;login;2AE-X;Клиент;7700000000;770001001;;\"2,000\";0;0;0;0;0;0;0;0;0;\r\n"
	rows, err := ParseEDOBilling([]byte(raw))
	if err != nil {
		t.Fatalf("ParseEDOBilling вернул ошибку: %v", err)
	}
	if rows[0].Limit == nil || *rows[0].Limit != 2 {
		t.Errorf("Limit = %v, ожидали 2", rows[0].Limit)
	}
}

// Дубль имени колонки в заголовке означает, что данные возьмутся не из того столбца.
func TestParseEDOBillingRejectsDuplicateColumn(t *testing.T) {
	// Все ожидаемые колонки на месте, но «Логин» продублирован в конце:
	// сейчас индекс перезаписывается и данные берутся из последнего столбца-тёзки.
	raw := strings.TrimSuffix(edoHeader, "\r\n") + ";Логин\r\n" +
		"99999;Партнёр;;login;2AE-X;Клиент;7700000000;770001001;;;0;0;0;0;0;0;0;0;0;;мусор\r\n"
	if _, err := ParseEDOBilling([]byte(raw)); err == nil {
		t.Fatal("ожидали ошибку о повторяющейся колонке")
	}
}

func TestParseEDOBillingRejectsUnknownHeader(t *testing.T) {
	if _, err := ParseEDOBilling([]byte("что-то;совсем;другое\r\n")); err == nil {
		t.Fatal("ожидали ошибку про неизвестный заголовок")
	}
}

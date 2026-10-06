package partner

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"partnerops/internal/money"
)

// Колонки отчёта биллинга ЭДО в том виде, в каком их отдаёт 1С.
var edoBillingColumns = []string{
	"Код партнера",
	"Название партнера",
	"Владелец",
	"Логин",
	"Ид_ЭДО клиента",
	"Наименование клиента",
	"ИНН клиента",
	"КПП клиента",
	"Тарифы ИТС",
	"Лимит",
	"СФ_исх",
	"не_СФ_исх",
	"Кол-во пакетов документов ЭДО",
	"Сумма пакетов документов ЭДО по владельцу",
	"Льгота",
	"Количество пакетов документов ЭДО к оплате",
	"Тариф для клиента",
	"Сумма для клиента",
	"Сумма для партнера",
	"Дополнительная информация",
}

// Колонки отчёта трафика ЭДО, нужные проверке связи идентификатора с логином.
// Отчёт шире (docs/API.md §8.2), остальные колонки здесь не требуются.
var edoTrafficColumns = []string{"Идентификатор ЭДО", "Логин"}

// readReportCSV читает CSV-отчёт 1С: разделитель — точка с запятой, в начале BOM.
func readReportCSV(raw []byte) ([][]string, error) {
	reader := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(raw, []byte("\ufeff"))))
	reader.Comma = ';'
	// Длину строк проверяем сами, чтобы отличить пустую строку от повреждённой.
	reader.FieldsPerRecord = -1

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("partner: не разобрать CSV: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("partner: пустой отчёт")
	}
	return records, nil
}

// ParseEDOTrafficLogins возвращает логины, у которых в отчёте трафика ЭДО
// стоит непустой идентификатор ЭДО. Порядок сохраняется, дубли убираются.
func ParseEDOTrafficLogins(raw []byte) ([]string, error) {
	records, err := readReportCSV(raw)
	if err != nil {
		return nil, err
	}

	index, err := indexColumns(records[0], edoTrafficColumns)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	logins := make([]string, 0, len(records)-1)
	for i, record := range records[1:] {
		if isBlank(record) {
			continue
		}
		// Строка другой длины — повреждённый отчёт. Пропустить её молча нельзя:
		// вышло бы ложное «связи нет».
		if len(record) != len(records[0]) {
			return nil, fmt.Errorf("partner: строка %d: полей %d, в заголовке %d",
				i+2, len(record), len(records[0]))
		}
		edoID := strings.TrimSpace(record[index["Идентификатор ЭДО"]])
		login := strings.TrimSpace(record[index["Логин"]])
		if edoID == "" || login == "" || seen[login] {
			continue
		}
		seen[login] = true
		logins = append(logins, login)
	}
	return logins, nil
}

// EDOTrafficRow — строка отчёта трафика ЭДО: один идентификатор за период.
// Взяты колонки, нужные подбору тарифа ЭПД; остальные см. docs/API.md §8.2.
type EDOTrafficRow struct {
	ClientName string
	INN        string
	KPP        string
	Subscriber string
	EDOID      string
	Login      string
	// EPDIn и EPDOut — электронные перевозочные документы, входящие и исходящие.
	EPDIn  int64
	EPDOut int64
	// InvoicesOut и NonInvoicesOut — исходящие счета-фактуры и прочие документы.
	// По живым данным «Кол-во пакетов документов ЭДО» биллинга совпадает с СФ_исх
	// (62 строки из 65), поэтому InvoicesOut — оценка пакетов для прогноза лимита.
	InvoicesOut    int64
	NonInvoicesOut int64
	InvoicesIn     int64
	NonInvoicesIn  int64
	// Operator — оператор ЭДО клиента: чужой оператор означает роуминг.
	Operator string
	// Даты — ГГГГ-ММ-ДД, пусто — 1С поле не заполнила. «Сопровождение» в живых
	// данных почти всегда пусто.
	SupportFrom  string
	SupportTo    string
	LinkCreated  string
	IDRegistered string
}

// Все 17 колонок отчёта трафика (docs/API.md §8.2, сверено с живым отчётом).
var edoTrafficUsageColumns = []string{
	"Название организации", "ИНН", "КПП", "Абонент", "Оператор",
	"Дата сопровождения с", "Дата сопровождения по", "Идентификатор ЭДО", "Логин",
	"Дата создания связи", "Дата регистрации идентификатора ЭДО",
	"СФ (вх.)", "не_СФ (вх.)", "ЭПД (вх.)", "СФ (исх.)", "не_СФ (исх.)", "ЭПД (исх.)",
}

// parseReportDate переводит дату отчёта «ДД.ММ.ГГГГ» в «ГГГГ-ММ-ДД».
// Пустая строка остаётся пустой.
func parseReportDate(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	// Иногда к дате приклеено время: «15.02.2017 0:00:00».
	day, _, _ := strings.Cut(s, " ")
	t, err := time.Parse("02.01.2006", day)
	if err != nil {
		return "", fmt.Errorf("дата %q не в формате ДД.ММ.ГГГГ", s)
	}
	return t.Format(time.DateOnly), nil
}

// ParseEDOTraffic разбирает отчёт трафика ЭДО по всем клиентам.
func ParseEDOTraffic(raw []byte) ([]EDOTrafficRow, error) {
	records, err := readReportCSV(raw)
	if err != nil {
		return nil, err
	}
	index, err := indexColumns(records[0], edoTrafficUsageColumns)
	if err != nil {
		return nil, err
	}

	rows := make([]EDOTrafficRow, 0, len(records)-1)
	for i, record := range records[1:] {
		if isBlank(record) {
			continue
		}
		if len(record) != len(records[0]) {
			return nil, fmt.Errorf("partner: строка %d: полей %d, в заголовке %d",
				i+2, len(record), len(records[0]))
		}
		get := func(column string) string { return strings.TrimSpace(record[index[column]]) }
		row := EDOTrafficRow{
			ClientName: get("Название организации"), INN: get("ИНН"), KPP: get("КПП"),
			Subscriber: get("Абонент"), EDOID: get("Идентификатор ЭДО"), Login: get("Логин"),
			Operator: get("Оператор"),
		}
		counts := []struct {
			column string
			target *int64
		}{
			{"ЭПД (вх.)", &row.EPDIn}, {"ЭПД (исх.)", &row.EPDOut},
			{"СФ (исх.)", &row.InvoicesOut}, {"не_СФ (исх.)", &row.NonInvoicesOut},
			{"СФ (вх.)", &row.InvoicesIn}, {"не_СФ (вх.)", &row.NonInvoicesIn},
		}
		for _, c := range counts {
			if *c.target, err = parseCount(get(c.column)); err != nil {
				return nil, fmt.Errorf("partner: строка %d, %s: %w", i+2, c.column, err)
			}
		}
		dates := []struct {
			column string
			target *string
		}{
			{"Дата сопровождения с", &row.SupportFrom}, {"Дата сопровождения по", &row.SupportTo},
			{"Дата создания связи", &row.LinkCreated}, {"Дата регистрации идентификатора ЭДО", &row.IDRegistered},
		}
		for _, d := range dates {
			if *d.target, err = parseReportDate(get(d.column)); err != nil {
				return nil, fmt.Errorf("partner: строка %d, %s: %w", i+2, d.column, err)
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// ParseEDOBilling разбирает CSV отчёта биллинга ЭДО.
func ParseEDOBilling(raw []byte) ([]EDOBillingRow, error) {
	records, err := readReportCSV(raw)
	if err != nil {
		return nil, err
	}

	index, err := indexColumns(records[0], edoBillingColumns)
	if err != nil {
		return nil, err
	}

	rows := make([]EDOBillingRow, 0, len(records)-1)
	for i, record := range records[1:] {
		if isBlank(record) {
			continue
		}
		// Строка другой длины — повреждённый отчёт, а не строка с пропусками.
		if len(record) != len(records[0]) {
			return nil, fmt.Errorf("partner: строка %d: полей %d, в заголовке %d",
				i+2, len(record), len(records[0]))
		}
		row, err := edoRowFrom(record, index)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// EPDBillingRow — строка выгрузки «Детализация биллинга» ЭПД с портала 1С-ЭДО.
// В партнёрском API отчёта биллинга ЭПД нет (docs/API.md §8.4), выгрузку
// загружают руками; из неё берутся реквизиты наших клиентов.
type EPDBillingRow struct {
	// Owner — «Владелец» как в выгрузке, после переноса с предыдущей строки.
	Owner string
	// OwnerCode и OwnerContact — части Owner: «FR-FR-1 - ФИО|почта|логин».
	OwnerCode    string
	OwnerContact string
	Login        string
	EDOID        string
	ClientName   string
	INN          string
	// KPP пуст у ИП.
	KPP        string
	ITSTariffs string
	// EPDDocs — «Кол-во документов ЭПД».
	EPDDocs int64
}

// Колонки выгрузки «Детализация биллинга» ЭПД, нужные реестру клиентов.
var epdBillingColumns = []string{
	"Владелец", "Логин", "Ид_ЭДО клиента", "Наименование клиента",
	"ИНН клиента", "КПП клиента", "Тарифы ИТС", "Кол-во документов ЭПД",
}

// ParseEPDBilling разбирает выгрузку «Детализация биллинга» ЭПД. Колонки ищутся
// по имени; кодировка — UTF-8 (с BOM или без) или Windows-1251, как сохраняет Excel.
//
// Пустой «Владелец» в этой выгрузке значит «тот же владелец, что строкой выше»:
// портал пишет владельца только в первой строке группы. В отчёте биллинга ЭДО
// пустой владелец значит иное — идентификатор не привязан (docs/API.md §8.1),
// поэтому перенос — только здесь.
func ParseEPDBilling(raw []byte) ([]EPDBillingRow, error) {
	if !utf8.Valid(raw) {
		raw = decodeCP1251(raw)
	}
	records, err := readReportCSV(raw)
	if err != nil {
		return nil, err
	}
	index, err := indexColumns(records[0], epdBillingColumns)
	if err != nil {
		return nil, err
	}
	rows := make([]EPDBillingRow, 0, len(records)-1)
	owner := ""
	for i, record := range records[1:] {
		if isBlank(record) {
			continue
		}
		if len(record) != len(records[0]) {
			return nil, fmt.Errorf("partner: строка %d: полей %d, в заголовке %d",
				i+2, len(record), len(records[0]))
		}
		get := func(column string) string { return strings.TrimSpace(record[index[column]]) }
		if value := get("Владелец"); value != "" {
			owner = value
		}
		row := EPDBillingRow{Owner: owner, Login: get("Логин"), EDOID: get("Ид_ЭДО клиента"),
			ClientName: get("Наименование клиента"), INN: get("ИНН клиента"), KPP: get("КПП клиента"),
			ITSTariffs: get("Тарифы ИТС")}
		row.OwnerCode, row.OwnerContact = ParseOwner(owner)
		if row.EPDDocs, err = parseCount(get("Кол-во документов ЭПД")); err != nil {
			return nil, fmt.Errorf("partner: строка %d, Кол-во документов ЭПД: %w", i+2, err)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// decodeCP1251 переводит текст Windows-1251 в UTF-8.
func decodeCP1251(raw []byte) []byte {
	var b strings.Builder
	b.Grow(len(raw) * 2)
	for _, c := range raw {
		switch {
		case c < 0x80:
			b.WriteByte(c)
		case c >= 0xC0:
			b.WriteRune(rune(c) - 0xC0 + 'А')
		case c == 0xA8:
			b.WriteRune('Ё')
		case c == 0xB8:
			b.WriteRune('ё')
		case c == 0xB9:
			b.WriteRune('№')
		case c == 0xA0:
			b.WriteRune(' ')
		case c == 0xAB:
			b.WriteRune('«')
		case c == 0xBB:
			b.WriteRune('»')
		case c == 0x96:
			b.WriteRune('–')
		case c == 0x97:
			b.WriteRune('—')
		default:
			b.WriteRune('�')
		}
	}
	return []byte(b.String())
}

// indexColumns сопоставляет имена колонок их позициям и требует, чтобы все ожидаемые были на месте.
func indexColumns(header []string, expected []string) (map[string]int, error) {
	index := make(map[string]int, len(header))
	for i, name := range header {
		name = strings.TrimSpace(name)
		// Дубль имени затирал бы индекс, и данные молча брались бы из тёзки.
		if _, ok := index[name]; ok {
			return nil, fmt.Errorf("partner: колонка %q в заголовке дважды", name)
		}
		index[name] = i
	}
	var missing []string
	for _, name := range expected {
		if _, ok := index[name]; !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("partner: в отчёте нет колонок: %s", strings.Join(missing, ", "))
	}
	return index, nil
}

func edoRowFrom(record []string, index map[string]int) (EDOBillingRow, error) {
	get := func(column string) string {
		i, ok := index[column]
		if !ok || i >= len(record) {
			return ""
		}
		return strings.TrimSpace(record[i])
	}

	row := EDOBillingRow{
		PartnerCode: get("Код партнера"),
		PartnerName: get("Название партнера"),
		Owner:       get("Владелец"),
		Login:       get("Логин"),
		EDOID:       get("Ид_ЭДО клиента"),
		ClientName:  get("Наименование клиента"),
		INN:         get("ИНН клиента"),
		KPP:         get("КПП клиента"),
		ITSTariffs:  get("Тарифы ИТС"),
		Extra:       get("Дополнительная информация"),
	}

	var err error
	if limit := get("Лимит"); limit != "" {
		value, parseErr := parseCount(limit)
		if parseErr != nil {
			return row, fmt.Errorf("partner: лимит %q: %w", limit, parseErr)
		}
		row.Limit = &value
	}

	counts := []struct {
		column string
		target *int64
	}{
		{"СФ_исх", &row.InvoicesOut},
		{"не_СФ_исх", &row.NonInvoicesOut},
		{"Кол-во пакетов документов ЭДО", &row.Packets},
		{"Сумма пакетов документов ЭДО по владельцу", &row.PacketsByOwner},
		{"Льгота", &row.Discount},
		{"Количество пакетов документов ЭДО к оплате", &row.PacketsBillable},
	}
	for _, c := range counts {
		if *c.target, err = parseCount(get(c.column)); err != nil {
			return row, fmt.Errorf("partner: колонка %q: %w", c.column, err)
		}
	}

	amounts := []struct {
		column string
		target *money.Amount
	}{
		{"Тариф для клиента", &row.TariffAmount},
		{"Сумма для клиента", &row.ClientAmount},
		{"Сумма для партнера", &row.PartnerAmount},
	}
	for _, a := range amounts {
		if *a.target, err = money.Parse(get(a.column)); err != nil {
			return row, fmt.Errorf("partner: колонка %q: %w", a.column, err)
		}
	}

	return row, nil
}

// parseCount разбирает счётчик документов. Пустое значение означает ноль.
func parseCount(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	// Счётчики иногда приходят с дробной частью вида "2,000" — её отбрасываем,
	// но только нулевую: "2,999" означает, что в поле не счётчик.
	if whole, frac, found := strings.Cut(s, ","); found {
		if strings.Trim(frac, "0") != "" {
			return 0, fmt.Errorf("ненулевая дробная часть в %q", s)
		}
		s = whole
	}
	return strconv.ParseInt(strings.ReplaceAll(s, " ", ""), 10, 64)
}

func isBlank(record []string) bool {
	for _, field := range record {
		if strings.TrimSpace(field) != "" {
			return false
		}
	}
	return true
}

package itsreq

import (
	"testing"
	"time"
)

func cellAt(cells []Cell, row, col int) (Cell, bool) {
	for _, cell := range cells {
		if cell.Row == row && cell.Col == col {
			return cell, true
		}
	}
	return Cell{}, false
}

func TestCellsFillHeader(t *testing.T) {
	cells := Cells(validRequest())

	partner, ok := cellAt(cells, 1, 4)
	if !ok || partner.Value != "00000" {
		t.Errorf("код партнёра не на месте: %+v", partner)
	}
	email, ok := cellAt(cells, 3, 4)
	if !ok || email.Value != "zakaz@example.ru" {
		t.Errorf("e-mail не на месте: %+v", email)
	}
}

func TestCellsFillFirstRowAtIndexTen(t *testing.T) {
	cells := Cells(validRequest())

	number, ok := cellAt(cells, 10, 0)
	if !ok || number.Value != "1" || !number.Number {
		t.Errorf("номер строки должен быть числом 1: %+v", number)
	}
	tariff, ok := cellAt(cells, 10, 4)
	if !ok || tariff.Value != "2083" {
		t.Errorf("вид 1С:ИТС: %+v", tariff)
	}
	inn, ok := cellAt(cells, 10, 7)
	if !ok || inn.Value != "7811000310" {
		t.Errorf("ИНН: %+v", inn)
	}
}

func TestCellsForceFixedValues(t *testing.T) {
	// Количество выпусков и способ оплаты для ЭПД заданы правилами,
	// вводить их руками нельзя.
	cells := Cells(validRequest())

	issues, ok := cellAt(cells, 10, 27)
	if !ok || issues.Value != "12" {
		t.Errorf("количество выпусков должно быть 12, получили %+v", issues)
	}
	payment, ok := cellAt(cells, 10, 28)
	if !ok || payment.Value != "1" {
		t.Errorf("способ оплаты должен быть 1, получили %+v", payment)
	}
	operation, ok := cellAt(cells, 10, 23)
	if !ok || operation.Value != "0" {
		t.Errorf("операция должна быть 0, получили %+v", operation)
	}
}

func TestCellsKeepCodesAsText(t *testing.T) {
	// «0» и «1» обязаны остаться строками: числом робот прочитает их иначе.
	cells := Cells(validRequest())

	for _, col := range []int{2, 23, 28} {
		cell, ok := cellAt(cells, 10, col)
		if !ok {
			t.Fatalf("колонка %d отсутствует", col)
		}
		if cell.Number {
			t.Errorf("колонка %d должна писаться текстом", col)
		}
	}
}

func TestCellsNumberSecondRow(t *testing.T) {
	request := validRequest()
	request.Rows = append(request.Rows, validRow())

	cells := Cells(request)

	second, ok := cellAt(cells, 11, 0)
	if !ok || second.Value != "2" {
		t.Errorf("вторая строка должна получить номер 2: %+v", second)
	}
}

// nextMonthCode — «Дата отказа» ближайшего допустимого месяца.
func nextMonthCode() string {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).
		AddDate(0, 1, 0).Format(monthLayout)
}

func TestCellsWriteRefusal(t *testing.T) {
	request := validRequest()
	request.Rows[0].Operation = OperationRefusal
	request.Rows[0].RefusalDate = nextMonthCode()
	request.Rows[0].RefusalReason = "4"

	cells := Cells(request)

	operation, ok := cellAt(cells, 10, 23)
	if !ok || operation.Value != "1" {
		t.Errorf("операция отказа должна быть 1: %+v", operation)
	}
	date, ok := cellAt(cells, 10, 24)
	if !ok || date.Value != nextMonthCode() {
		t.Errorf("дата отказа: %+v", date)
	}
	reason, ok := cellAt(cells, 10, 25)
	if !ok || reason.Value != "4" {
		t.Errorf("причина отказа: %+v", reason)
	}
}

func TestCellsNewOperationKeepsRefusalColumnsEmpty(t *testing.T) {
	// Поля отказа могли остаться в форме от прежнего выбора: в файл они не идут.
	request := validRequest()
	request.Rows[0].RefusalDate = "01.12"
	request.Rows[0].RefusalReason = "3"

	cells := Cells(request)

	operation, ok := cellAt(cells, 10, 23)
	if !ok || operation.Value != OperationNew {
		t.Errorf("операция нового договора должна быть 0: %+v", operation)
	}
	for _, col := range []int{24, 25} {
		if cell, ok := cellAt(cells, 10, col); ok {
			t.Errorf("колонка %d при новом договоре должна остаться пустой: %+v", col, cell)
		}
	}
}

func TestCellsWriteExtraRegNumbers(t *testing.T) {
	request := validRequest()
	// Пустой слот формы не должен оставлять дыру в колонках 29–32.
	request.Rows[0].ExtraRegNumbers = []string{"11111111", "", "22222222", "33333333", "44444444", "55555555"}

	cells := Cells(request)

	want := map[int]string{29: "11111111", 30: "22222222", 31: "33333333", 32: "44444444"}
	for col, value := range want {
		cell, ok := cellAt(cells, 10, col)
		if !ok || cell.Value != value {
			t.Errorf("колонка %d: ожидали %q, получили %+v", col, value, cell)
		}
	}
	if cell, ok := cellAt(cells, 10, 33); ok {
		t.Errorf("пятый номер писать некуда, колонка 33 — регномер апгрейда: %+v", cell)
	}
}

func TestCellsWriteHeaderPasswords(t *testing.T) {
	request := validRequest()
	request.Password = "Secret2026"
	request.NewPassword = "Secret2027"

	cells := Cells(request)

	password, ok := cellAt(cells, 1, 8)
	if !ok || password.Value != "Secret2026" {
		t.Errorf("пароль: %+v", password)
	}
	newPassword, ok := cellAt(cells, 2, 8)
	if !ok || newPassword.Value != "Secret2027" {
		t.Errorf("новый пароль: %+v", newPassword)
	}
}

func TestCellsOmitEmptyOptionalFields(t *testing.T) {
	cells := Cells(validRequest())

	// Факс не заполнен — пустую ячейку писать незачем.
	if _, ok := cellAt(cells, 10, 21); ok {
		t.Error("пустой факс не должен попадать в файл")
	}
}

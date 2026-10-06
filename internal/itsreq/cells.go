package itsreq

import (
	"strconv"
	"strings"

	"partnerops/internal/xlsfill"
)

// firstDataRow — строка шаблона, с которой начинается таблица регистрации.
// Нумерация с нуля, как в самом файле.
const firstDataRow = 10

// MaxRows — предел строк таблицы регистрации в одном файле: строки 11–32
// шаблона. Больше клиентов — несколько файлов.
const MaxRows = 22

// firstExtraRegCol — первая из колонок 29–32 «Регистрационные номера 1–4».
const firstExtraRegCol = 29

// Колонки полей ввода шапки: код партнёра, ответственный и e-mail — в E:F,
// пароль и новый пароль — в I.
const (
	headerValueCol    = 4
	headerPasswordCol = 8
)

// Cell — одна ячейка шаблона. Number ставится только там, где робот ждёт число.
type Cell = xlsfill.Cell

// Cells раскладывает заявку по ячейкам шаблона.
//
// Всё пишется текстом намеренно: коды «0» и «1», даты «01.10.26» и номера
// с ведущими нулями, став числами, будут прочитаны роботом иначе.
// Пустые необязательные поля пропускаются — шаблон уже содержит нужное оформление.
func Cells(request Request) []Cell {
	cells := make([]Cell, 0, 32)

	put := func(row, col int, value string) {
		if value == "" {
			return
		}
		cells = append(cells, Cell{Row: row, Col: col, Value: value})
	}

	// Шапка «Информация об отправителе заявки». Подписи слева — объединённые
	// A:D и G:H; значение, записанное внутрь объединения, скрыто и роботом
	// не читается. Поля ввода — E:F (колонка 4) и I (колонка 8).
	put(1, headerValueCol, request.PartnerCode)
	put(1, headerPasswordCol, request.Password)
	put(2, headerValueCol, request.Responsible)
	put(2, headerPasswordCol, request.NewPassword)
	put(3, headerValueCol, request.Email)

	for index, row := range request.Rows {
		line := firstDataRow + index

		// «№ п/п» в шаблоне — число, в отличие от кодов ниже.
		cells = append(cells, Cell{Row: line, Col: 0, Value: strconv.Itoa(index + 1), Number: true})
		put(line, 1, request.PartnerCode)
		put(line, 2, row.DeliveryType)
		put(line, 3, row.DistributorCode)
		put(line, 4, row.TariffCode)
		put(line, 5, row.RegNumber)
		put(line, 6, row.CompanyName)
		put(line, 7, row.INN)
		put(line, 8, row.KPP)
		if row.Workplaces > 0 {
			put(line, 9, strconv.Itoa(row.Workplaces))
		}
		put(line, 10, row.ActivityType)
		put(line, 11, row.Director)
		put(line, 12, row.Responsible)
		put(line, 13, row.PostalCode)
		put(line, 14, row.City)
		put(line, 15, row.Street)
		put(line, 16, row.House)
		put(line, 17, row.Building)
		put(line, 18, row.Flat)
		put(line, 19, row.PhoneCode)
		put(line, 20, row.Phone)
		put(line, 21, row.Fax)
		put(line, 22, row.Email)
		put(line, 23, row.operation())
		// Дата и причина отказа имеют смысл только при отказе: у нового договора
		// заполненные поля формы в файл не переносятся.
		if row.operation() == OperationRefusal {
			put(line, 24, row.RefusalDate)
			put(line, 25, row.RefusalReason)
		}
		put(line, 26, row.StartDate)
		put(line, 27, IssuesCount)
		put(line, 28, PaymentPrepaid)

		// Пустые слоты формы пропускаем, чтобы номера легли подряд.
		column := firstExtraRegCol
		for _, number := range row.ExtraRegNumbers {
			if column >= firstExtraRegCol+maxExtraRegNumbers {
				break
			}
			if strings.TrimSpace(number) == "" {
				continue
			}
			put(line, column, number)
			column++
		}
	}

	return cells
}

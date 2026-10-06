// Package itsreq собирает файл-заявку на регистрацию тарифов 1С-ЭПД.
//
// Правила и таблицы кодов взяты из инфовыпуска «1С» № 34677 от 17.07.2026
// и «Справочника партнёра по ИТС», см. docs/REQUESTS.md.
package itsreq

import (
	"regexp"
	"strconv"
)

// Tariff — тариф 1С-ЭПД, доступный к оформлению файлом-заявкой.
type Tariff struct {
	// Code идёт в колонку «Вид 1С:ИТС».
	Code string
	Name string
	// Volume — число документов в пакете за год.
	Volume int
	// Nomenclature — номенклатурный номер, нужен для счёта.
	Nomenclature string
	// RetailKopeks и PartnerKopeks хранятся в копейках: цены целые, дробей не бывает.
	RetailKopeks  int64
	PartnerKopeks int64
	// FreshCode — код того же объёма в «Менеджере сервиса» для облака Фреш.
	// В файле-заявке он недопустим, но пригодится в подсказке.
	FreshCode string
}

// Tariffs перечислены по возрастанию объёма.
var Tariffs = []Tariff{
	{"2092", "1С-ЭДО. ЭПД-200", 200, "2900004041864", 140000, 84000, "1223"},
	{"2080", "1С-ЭДО. ЭПД-600", 600, "2900002632743", 360000, 216000, "1200"},
	{"2081", "1С-ЭДО. ЭПД-1000", 1000, "2900002632750", 500000, 300000, "1201"},
	{"2093", "1С-ЭДО. ЭПД-2000", 2000, "2900004041871", 950000, 570000, "1224"},
	{"2094", "1С-ЭДО. ЭПД-3000", 3000, "2900004041888", 1400000, 840000, "1225"},
	{"2095", "1С-ЭДО. ЭПД-4000", 4000, "2900004041895", 1850000, 1110000, "1226"},
	{"2082", "1С-ЭДО. ЭПД-5000", 5000, "2900002632767", 2250000, 1350000, "1202"},
	{"2096", "1С-ЭДО. ЭПД-7000", 7000, "2900004041901", 2950000, 1770000, "1227"},
	{"2083", "1С-ЭДО. ЭПД-10000", 10000, "2900002632774", 4000000, 2400000, "1203"},
	{"2840", "1С-ЭДО. ЭПД-20000", 20000, "2900004041918", 7000000, 4200000, "1228"},
	{"2841", "1С-ЭДО. ЭПД-30000", 30000, "2900004041925", 9600000, 5760000, "1229"},
	{"2084", "1С-ЭДО. ЭПД-50000", 50000, "2900002632781", 15000000, 9000000, "1204"},
	{"2085", "1С-ЭДО. ЭПД-100000", 100000, "2900002632798", 25000000, 15000000, "1205"},
}

// PerPieceRetailKopeks — розничная цена ЭПД поштучно, без тарифа
// (номенклатура 2900002632736, инфовыпуск № 34677, docs/REQUESTS.md §6).
const PerPieceRetailKopeks int64 = 700

// IndividualVolume — с этого годового объёма тариф индивидуальный, по запросу
// на epd@1c.ru (docs/REQUESTS.md §6).
const IndividualVolume int64 = 200000

// YearCostKopeks — розничная стоимость года ЭПД при объёме yearly: пакеты
// packagesKopeks покрывают covered документов, сверх них — поштучно.
func YearCostKopeks(yearly, covered, packagesKopeks int64) int64 {
	return packagesKopeks + max(0, yearly-covered)*PerPieceRetailKopeks
}

// BestEPDTariff подбирает самый дешёвый в рознице способ оплатить yearly
// документов ЭПД за год: поштучно или один тариф (остаток сверх пакета —
// поштучно). ok false — выгоднее поштучно; при равной цене пакет не
// предлагается.
func BestEPDTariff(yearly int64) (best Tariff, ok bool, costKopeks int64) {
	costKopeks = YearCostKopeks(yearly, 0, 0)
	for _, tariff := range Tariffs {
		cost := YearCostKopeks(yearly, int64(tariff.Volume), tariff.RetailKopeks)
		if cost < costKopeks {
			best, ok, costKopeks = tariff, true, cost
		}
	}
	return best, ok, costKopeks
}

// TariffByCode ищет тариф по коду вида 1С:ИТС.
func TariffByCode(code string) (Tariff, bool) {
	for _, tariff := range Tariffs {
		if tariff.Code == code {
			return tariff, true
		}
	}
	return Tariff{}, false
}

// epdInText находит тариф ЭПД в колонке «Тарифы ИТС» отчёта биллинга:
// «1С-ЭДО. ЭПД-600(0): подп. ИТС №: …», «1С-ЭДО. ЭПД-1000(0)x2: …».
var epdInText = regexp.MustCompile(`ЭПД-(\d+)\(\d+\)(?:x(\d+))?`)

// EPDHolding — тариф ЭПД, который уже есть у клиента, и число подписок на него.
type EPDHolding struct {
	Tariff Tariff
	Count  int
}

// EPDTariffsInText возвращает тарифы ЭПД из колонки «Тарифы ИТС». Объём,
// которого нет в справочнике, пропускается: выдумывать ему цену нельзя.
func EPDTariffsInText(text string) []EPDHolding {
	var list []EPDHolding
	for _, match := range epdInText.FindAllStringSubmatch(text, -1) {
		volume, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}
		count := 1
		if match[2] != "" {
			if n, err := strconv.Atoi(match[2]); err == nil && n > 0 {
				count = n
			}
		}
		for _, tariff := range Tariffs {
			if tariff.Volume == volume {
				list = append(list, EPDHolding{Tariff: tariff, Count: count})
				break
			}
		}
	}
	return list
}

// FreshCodeForVolume возвращает код для «Менеджера сервиса» по объёму пакета.
func FreshCodeForVolume(volume int) (string, bool) {
	for _, tariff := range Tariffs {
		if tariff.Volume == volume {
			return tariff.FreshCode, true
		}
	}
	return "", false
}

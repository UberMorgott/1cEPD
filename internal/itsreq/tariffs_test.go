package itsreq

import "testing"

func TestTariffsCoverAllThirteenCodes(t *testing.T) {
	if len(Tariffs) != 13 {
		t.Fatalf("тарифов %d, ожидали 13", len(Tariffs))
	}
	for _, code := range []string{
		"2092", "2080", "2081", "2093", "2094", "2095", "2082",
		"2096", "2083", "2840", "2841", "2084", "2085",
	} {
		if _, ok := TariffByCode(code); !ok {
			t.Errorf("код %s отсутствует в справочнике", code)
		}
	}
}

func TestEPDTariffsInTextReadsBillingColumn(t *testing.T) {
	// Формы из живой колонки «Тарифы ИТС»: одна подписка, две подписки, без ЭПД.
	got := EPDTariffsInText("1С-ЭДО. ЭПД-1000(0)x2: подп. ИТС №: 1, 2 | 1С:ИТС Базовый(50): подп. ИТС №: 3")
	if len(got) != 1 || got[0].Tariff.Code != "2081" || got[0].Count != 2 {
		t.Errorf("ЭПД-1000 x2: %+v", got)
	}
	if got := EPDTariffsInText("1С-ЭДО. ЭПД-600(0): подп. ИТС №: 1"); len(got) != 1 || got[0].Count != 1 {
		t.Errorf("ЭПД-600: %+v", got)
	}
	if got := EPDTariffsInText("ИТСааС ПРОФ(100): подп. Фреш: 1 | 1С-ЭДО. ЭПД-999(0): x"); len(got) != 0 {
		t.Errorf("неизвестный объём или нет ЭПД: %+v", got)
	}
}

func TestBestEPDTariffPicksCheapestRetail(t *testing.T) {
	cases := []struct {
		yearly int64
		code   string // пусто — поштучно
		cost   int64
	}{
		{0, "", 0},
		{150, "", 105000},                      // 150 × 7,00 дешевле ЭПД-200 за 1 400
		{1000, "2081", 500000},                 // ЭПД-1000 ровно
		{2500, "2093", 1300000},                // ЭПД-2000 + 500 поштучно = 9 500 + 3 500
		{120000, "2085", 25000000 + 20000*700}, // ЭПД-100000 + остаток поштучно
	}
	for _, c := range cases {
		best, ok, cost := BestEPDTariff(c.yearly)
		code := ""
		if ok {
			code = best.Code
		}
		if code != c.code || cost != c.cost {
			t.Errorf("%d документов: %q за %d, ожидали %q за %d", c.yearly, code, cost, c.code, c.cost)
		}
	}
}

func TestTariffByCodeRejectsUnknown(t *testing.T) {
	// Коды из Менеджера сервиса (Фреш) в файле-заявке недопустимы.
	for _, code := range []string{"1223", "1200", "9999", ""} {
		if _, ok := TariffByCode(code); ok {
			t.Errorf("код %s не должен приниматься", code)
		}
	}
}

func TestTariffCarriesVolumeAndPrices(t *testing.T) {
	tariff, ok := TariffByCode("2083")
	if !ok {
		t.Fatal("код 2083 не найден")
	}
	if tariff.Name != "1С-ЭДО. ЭПД-10000" {
		t.Errorf("название %q", tariff.Name)
	}
	if tariff.Volume != 10000 {
		t.Errorf("объём %d, ожидали 10000", tariff.Volume)
	}
	if tariff.Nomenclature != "2900002632774" {
		t.Errorf("номенклатура %q", tariff.Nomenclature)
	}
	if tariff.RetailKopeks != 4000000 {
		t.Errorf("рекомендованная розница %d, ожидали 4000000", tariff.RetailKopeks)
	}
	if tariff.PartnerKopeks != 2400000 {
		t.Errorf("цена партнёра %d, ожидали 2400000", tariff.PartnerKopeks)
	}
}

func TestFreshCodesAreListedSeparately(t *testing.T) {
	// Для облака Фреш тариф оформляется подпиской, а не заявкой.
	// Справочник нужен, чтобы подсказать сотруднику правильный код.
	code, ok := FreshCodeForVolume(10000)
	if !ok || code != "1203" {
		t.Errorf("код Фреш для 10000 = %q, ожидали 1203", code)
	}
}

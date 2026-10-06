package xlsfill

import (
	"bytes"
	"os"
	"slices"
	"strings"
	"testing"
)

const templatePath = "../itsreq/ip00000_ru.xls"

func template(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("шаблон: %v", err)
	}
	return raw
}

func fill(t *testing.T, cells ...Cell) []byte {
	t.Helper()
	out, err := Fill(template(t), cells)
	if err != nil {
		t.Fatalf("Fill: %v", err)
	}
	return out
}

func read(t *testing.T, data []byte, sheet int) map[[2]int]Value {
	t.Helper()
	cells, err := ReadCells(data, sheet)
	if err != nil {
		t.Fatalf("ReadCells: %v", err)
	}
	return cells
}

func workbookOf(t *testing.T, data []byte) *workbook {
	t.Helper()
	f, err := readCFB(data)
	if err != nil {
		t.Fatal(err)
	}
	idx, _ := f.stream("Workbook")
	wb, err := parseWorkbook(f.streams[idx])
	if err != nil {
		t.Fatal(err)
	}
	return wb
}

// Без записей поток Workbook собирается байт в байт: пересчёт DBCELL, INDEX,
// EXTSST и BOUNDSHEET совпадает с тем, что записал Excel.
func TestRebuildIsByteIdentical(t *testing.T) {
	f, err := readCFB(template(t))
	if err != nil {
		t.Fatal(err)
	}
	idx, _ := f.stream("Workbook")
	orig := f.streams[idx]
	wb, err := parseWorkbook(orig)
	if err != nil {
		t.Fatal(err)
	}
	for i := range wb.sheets {
		if wb.sheets[i], err = patchSheet(wb.sheets[i], nil); err != nil {
			t.Fatal(err)
		}
	}
	got, err := wb.bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, orig) {
		t.Fatal("пересобранный поток Workbook отличается от шаблона")
	}

	container, err := f.bytes()
	if err != nil {
		t.Fatal(err)
	}
	again, err := readCFB(container)
	if err != nil {
		t.Fatal(err)
	}
	for i, s := range f.streams {
		if !bytes.Equal(again.streams[i], s) {
			t.Errorf("поток %q изменился при пересборке OLE2", entryName(f.entries[i]))
		}
	}
}

// Структуру шаблона менять нельзя: листы, оформление и справочник должны уцелеть.
func TestPreservesTemplateStructure(t *testing.T) {
	src := workbookOf(t, template(t))
	out := workbookOf(t, fill(t, Cell{Row: 1, Col: 4, Value: "00000"}))

	if len(out.sheets) != len(src.sheets) {
		t.Fatalf("листов %d, в шаблоне %d", len(out.sheets), len(src.sheets))
	}
	// Все записи глобального подпотока, кроме SST и смещений, — байт в байт:
	// стили (XF, FONT, FORMAT, STYLE), палитра, имена листов.
	skip := func(r record) bool {
		return r.id == recSST || r.id == recContinue || r.id == recExtSST || r.id == recBoundSheet
	}
	a := slices.DeleteFunc(slices.Clone(src.globals), skip)
	b := slices.DeleteFunc(slices.Clone(out.globals), skip)
	if len(a) != len(b) {
		t.Fatalf("записей в глобальном подпотоке %d, было %d", len(b), len(a))
	}
	for i := range a {
		if a[i].id != b[i].id || !bytes.Equal(a[i].data, b[i].data) {
			t.Fatalf("запись %d (0x%04X) глобального подпотока изменилась", i, a[i].id)
		}
	}
	for i := range src.globals {
		if src.globals[i].id == recBoundSheet {
			j := slices.IndexFunc(out.globals, func(r record) bool { return r.id == recBoundSheet })
			if !bytes.Equal(src.globals[i].data[4:], out.globals[j].data[4:]) {
				t.Error("имя или тип листа изменились")
			}
			break
		}
	}
	// Справочник «Коды ИТС_сервисов» и прочие листы не тронуты, кроме INDEX.
	for s := 1; s < len(src.sheets); s++ {
		if len(src.sheets[s]) != len(out.sheets[s]) {
			t.Fatalf("лист %d: записей %d, было %d", s, len(out.sheets[s]), len(src.sheets[s]))
		}
		for i, r := range src.sheets[s] {
			if r.id != recIndex && !bytes.Equal(r.data, out.sheets[s][i].data) {
				t.Fatalf("лист %d: запись %d (0x%04X) изменилась", s, i, r.id)
			}
		}
	}
	if !slices.Equal(out.strings[:len(src.strings)], src.strings) {
		t.Error("строки шаблона в SST изменились")
	}
}

func TestWritesValues(t *testing.T) {
	cells := read(t, fill(t,
		Cell{Row: 10, Col: 4, Value: "2092"},
		Cell{Row: 10, Col: 6, Value: `ООО "Тест"`},
		Cell{Row: 10, Col: 9, Value: "12", Number: true},
	), 0)

	if v := cells[[2]int{10, 4}]; v.IsNumber || v.Text != "2092" {
		t.Errorf("E11 = %+v, ожидали текст 2092", v)
	}
	if v := cells[[2]int{10, 6}]; v.Text != `ООО "Тест"` {
		t.Errorf("G11 = %+v", v)
	}
	if v := cells[[2]int{10, 9}]; !v.IsNumber || v.Number != 12 {
		t.Errorf("J11 = %+v, ожидали число 12", v)
	}
}

// formatOf возвращает строку формата XF: встроенный 49 — «@» (Текст).
func formatOf(t *testing.T, wb *workbook, xf int) string {
	t.Helper()
	n := 0
	for _, r := range wb.globals {
		if r.id != recXF {
			continue
		}
		if n == xf {
			ifmt := int(le.Uint16(r.data[2:]))
			if ifmt == 49 {
				return "@"
			}
			for _, f := range wb.globals {
				if f.id == recFormat && int(le.Uint16(f.data)) == ifmt {
					return string(f.data[5:]) // для ASCII-формата достаточно
				}
			}
			return "builtin"
		}
		n++
	}
	t.Fatalf("XF %d нет в шаблоне", xf)
	return ""
}

// Робот сверяет оформление: запись значения не должна менять XF ячейки.
// E2/I2/E3 — поля шапки, A11 — номер строки, G11/AA11 — текстовые (@) колонки.
func TestKeepsTemplateCellFormat(t *testing.T) {
	coords := [][2]int{{1, 4}, {1, 8}, {2, 4}, {3, 4}, {2, 8}, {10, 0}, {10, 6}, {10, 26}, {31, 6}}
	var cells []Cell
	for _, c := range coords {
		cells = append(cells, Cell{Row: c[0], Col: c[1], Value: "x"})
	}
	out := fill(t, cells...)

	before, after := effectiveXF(t, template(t)), read(t, out, 0)
	for _, c := range coords {
		if before(c) != after[c].XF {
			t.Errorf("%v: XF %d, в шаблоне %d", c, after[c].XF, before(c))
		}
		if after[c].Text != "x" {
			t.Errorf("%v: значение %+v", c, after[c])
		}
	}
	if got := formatOf(t, workbookOf(t, out), after[[2]int{10, 6}].XF); got != "@" {
		t.Errorf("формат G11 = %q, ожидали @", got)
	}
	// Соседи заменённой ячейки из того же MULBLANK сохранили своё оформление.
	got := effectiveXF(t, out)
	for col := range 35 {
		c := [2]int{10, col}
		if before(c) != got(c) {
			t.Errorf("%v: XF соседа %d, в шаблоне %d", c, got(c), before(c))
		}
	}
}

func TestNumberCellKeepsFormat(t *testing.T) {
	out := fill(t, Cell{Row: 10, Col: 0, Value: "1", Number: true}, Cell{Row: 11, Col: 0, Value: "2", Number: true})
	before, after := effectiveXF(t, template(t)), read(t, out, 0)
	for i, row := range []int{10, 11} {
		v := after[[2]int{row, 0}]
		if !v.IsNumber || v.Number != float64(i+1) {
			t.Errorf("A%d = %+v, ожидали число %d", row+1, v, i+1)
		}
		if v.XF != before([2]int{row, 0}) {
			t.Errorf("A%d: XF %d, в шаблоне %d", row+1, v.XF, before([2]int{row, 0}))
		}
	}
}

// Кириллица, кавычки, «ёлочки» и символы вне BMP доходят до файла без искажений.
func TestCyrillicRoundTrip(t *testing.T) {
	text := `ООО «Ёлка» "Тест" — Щукин Ю.Ж. 𝄞`
	cells := read(t, fill(t, Cell{Row: 10, Col: 6, Value: text}), 0)
	if got := cells[[2]int{10, 6}].Text; got != text {
		t.Errorf("G11 = %q, ожидали %q", got, text)
	}
}

// Длинные строки переходят через границу записи CONTINUE и читаются целиком.
func TestLongStringsSpanContinue(t *testing.T) {
	long := strings.Repeat("Щ", 9000)
	var in []Cell
	for row := 10; row < 32; row++ {
		in = append(in, Cell{Row: row, Col: 6, Value: long + string(rune('А'+row))})
	}
	cells := read(t, fill(t, in...), 0)
	for _, c := range in {
		if got := cells[[2]int{c.Row, c.Col}].Text; got != c.Value {
			t.Fatalf("строка %d: длина %d, ожидали %d", c.Row, len(got), len(c.Value))
		}
	}
}

// Код «0» обязан остаться строкой: способ оплаты и операция читаются как текст.
func TestKeepsCodesAsText(t *testing.T) {
	v := read(t, fill(t, Cell{Row: 10, Col: 23, Value: "0"}), 0)[[2]int{10, 23}]
	if v.IsNumber || v.Text != "0" {
		t.Errorf("X11 = %+v, ожидали текст 0", v)
	}
}

func TestRepeatedValueSharesString(t *testing.T) {
	out := fill(t, Cell{Row: 1, Col: 4, Value: "00000"}, Cell{Row: 10, Col: 1, Value: "00000"})
	if got, want := len(workbookOf(t, out).strings), len(workbookOf(t, template(t)).strings)+1; got != want {
		t.Errorf("строк в SST %d, ожидали %d", got, want)
	}
}

func TestRejectsBadCells(t *testing.T) {
	for name, cell := range map[string]Cell{
		"нет листа":           {Sheet: 99, Value: "x"},
		"строка вне .xls":     {Row: 70000, Value: "x"},
		"столбец вне .xls":    {Col: 300, Value: "x"},
		"не число":            {Row: 10, Col: 9, Value: "не число", Number: true},
		"длинный текст":       {Row: 10, Col: 6, Value: strings.Repeat("я", MaxText+1)},
		"скрыта объединением": {Row: 1, Col: 1, Value: "00000"}, // B2 внутри A2:D2
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Fill(template(t), []Cell{cell}); err == nil {
				t.Error("ожидали ошибку")
			}
		})
	}
}

func TestRejectsNotXLS(t *testing.T) {
	if _, err := Fill([]byte("не настоящий BIFF8"), nil); err == nil {
		t.Error("ожидали ошибку на не-.xls")
	}
}

// effectiveXF — оформление ячейки листа так, как его видит Excel: XF записи
// ячейки, а у пустой — XF строки или столбца.
func effectiveXF(t *testing.T, data []byte) func([2]int) int {
	t.Helper()
	cells := read(t, data, 0)
	recs := workbookOf(t, data).sheets[0]
	return func(c [2]int) int {
		if v, ok := cells[c]; ok {
			return v.XF
		}
		var row []byte
		for _, r := range recs {
			if r.id == recRow && int(le.Uint16(r.data)) == c[0] {
				row = r.data
			}
		}
		return defaultXF(row, recs, c[1])
	}
}

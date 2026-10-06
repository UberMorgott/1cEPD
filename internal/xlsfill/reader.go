package xlsfill

import (
	"errors"
	"fmt"
	"math"
)

// Value — прочитанная ячейка: значение и индекс XF её оформления.
type Value struct {
	Text     string
	Number   float64
	IsNumber bool
	XF       int
}

// ReadCells читает значения ячеек листа .xls — для проверки готового файла.
// Ключ — {строка, столбец} с нуля. Пустые ячейки с оформлением (BLANK)
// попадают в результат с пустым Text.
func ReadCells(data []byte, sheet int) (map[[2]int]Value, error) {
	f, err := readCFB(data)
	if err != nil {
		return nil, err
	}
	stream, ok := f.stream("Workbook")
	if !ok {
		return nil, errors.New("в файле нет потока Workbook")
	}
	wb, err := parseWorkbook(f.streams[stream])
	if err != nil {
		return nil, err
	}
	if sheet < 0 || sheet >= len(wb.sheets) {
		return nil, fmt.Errorf("листа %d нет", sheet)
	}

	out := map[[2]int]Value{}
	for _, r := range wb.sheets[sheet] {
		if !isCell(r.id) || len(r.data) < 6 {
			continue
		}
		row, col, xf := cellRef(r)
		d := r.data
		switch r.id {
		case recLabelSST:
			idx := int(le.Uint32(d[6:]))
			if idx >= len(wb.strings) {
				return nil, fmt.Errorf("ячейка (%d,%d) ссылается на строку %d вне SST", row, col, idx)
			}
			out[[2]int{row, col}] = Value{Text: wb.strings[idx], XF: xf}
		case recNumber:
			out[[2]int{row, col}] = Value{Number: math.Float64frombits(le.Uint64(d[6:])), IsNumber: true, XF: xf}
		case recRK:
			out[[2]int{row, col}] = Value{Number: rkValue(le.Uint32(d[6:])), IsNumber: true, XF: xf}
		case recBlank:
			out[[2]int{row, col}] = Value{XF: xf}
		case recMulBlank:
			for c := col; 6+2*(c-col) <= len(d)-2; c++ {
				out[[2]int{row, c}] = Value{XF: int(le.Uint16(d[4+2*(c-col):]))}
			}
		case recMulRK:
			for c := col; 10+6*(c-col) <= len(d)-2; c++ {
				k := 4 + 6*(c-col)
				out[[2]int{row, c}] = Value{Number: rkValue(le.Uint32(d[k+2:])), IsNumber: true, XF: int(le.Uint16(d[k:]))}
			}
		default:
			// LABEL, формулы и прочее заполнению не нужны; оформление сохраняем.
			out[[2]int{row, col}] = Value{XF: xf}
		}
	}
	return out, nil
}

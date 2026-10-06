// Package xlsfill вписывает значения в готовый шаблон .xls (BIFF8).
//
// Библиотек, которые пишут BIFF8 и сохраняют оформление шаблона, для Go нет,
// поэтому файл правится на уровне записей: меняются только ячейки заявки,
// таблица общих строк (SST) дополняется новыми строками, а смещения,
// которые от этого сдвигаются (BOUNDSHEET, INDEX, DBCELL, EXTSST),
// пересчитываются. Остальные записи, включая стили, переносятся байт в байт.
// Каждая ячейка сохраняет XF шаблона: формат («Текст»), рамки, шрифт и
// выравнивание остаются прежними.
package xlsfill

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"unicode/utf16"
)

// Пределы формата BIFF8.
const (
	MaxRows = 65536
	MaxCols = 256
	MaxText = 32767
)

// Cell — одно значение для записи в шаблон. Индексы с нуля, как в файле.
type Cell struct {
	Sheet int
	Row   int
	Col   int
	Value string
	// Number пишет Value числом; иначе значение остаётся текстом, даже «0».
	Number bool
}

// write — подготовленная запись одной ячейки.
type write struct {
	index    int // номер во входном списке, для сообщений
	row, col int
	number   bool
	num      float64
	sst      int // индекс строки в SST
}

// Fill возвращает содержимое .xls: шаблон с вписанными значениями.
// Шаблон не меняется.
func Fill(template []byte, cells []Cell) (_ []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			layout, ok := r.(errLayout)
			if !ok {
				panic(r)
			}
			err = layout
		}
	}()

	f, err := readCFB(template)
	if err != nil {
		return nil, err
	}
	stream, ok := f.stream("Workbook")
	if !ok {
		return nil, errors.New("в файле нет потока Workbook: нужен .xls Excel 97–2003")
	}
	wb, err := parseWorkbook(f.streams[stream])
	if err != nil {
		return nil, err
	}

	bySheet := map[int][]write{}
	var added []string
	known := map[string]int{}
	for i, c := range cells {
		w, err := prepare(c, i, len(wb.sheets))
		if err != nil {
			return nil, err
		}
		if !w.number {
			idx, seen := known[c.Value]
			if !seen {
				idx = len(wb.strings) + len(added)
				known[c.Value] = idx
				added = append(added, c.Value)
			}
			w.sst = idx
		}
		bySheet[c.Sheet] = append(bySheet[c.Sheet], w)
	}

	for sheet, writes := range bySheet {
		patched, err := patchSheet(wb.sheets[sheet], writes)
		if err != nil {
			return nil, err
		}
		wb.sheets[sheet] = patched
	}

	refs := 0
	for _, writes := range bySheet {
		for _, w := range writes {
			if !w.number {
				refs++
			}
		}
	}
	if len(added) > 0 {
		if wb.globals, err = appendSST(wb.globals, added, refs); err != nil {
			return nil, err
		}
	}

	out, err := wb.bytes()
	if err != nil {
		return nil, err
	}
	f.streams[stream] = out
	return f.bytes()
}

func prepare(c Cell, i, sheets int) (write, error) {
	w := write{index: i, row: c.Row, col: c.Col, number: c.Number}
	switch {
	case c.Sheet < 0 || c.Sheet >= sheets:
		return w, fmt.Errorf("ячейка %d: лист %d отсутствует, в шаблоне их %d", i, c.Sheet, sheets)
	case c.Row < 0 || c.Row >= MaxRows:
		return w, fmt.Errorf("ячейка %d: строка %d вне диапазона формата .xls", i, c.Row)
	case c.Col < 0 || c.Col >= MaxCols:
		return w, fmt.Errorf("ячейка %d: столбец %d вне диапазона формата .xls", i, c.Col)
	}
	if c.Number {
		v, err := strconv.ParseFloat(c.Value, 64)
		if err != nil || math.IsInf(v, 0) || math.IsNaN(v) {
			return w, fmt.Errorf("ячейка %d: ожидалось число, а не %q", i, c.Value)
		}
		w.num = v
		return w, nil
	}
	if len(utf16.Encode([]rune(c.Value))) > MaxText {
		return w, fmt.Errorf("ячейка %d: текст длиннее %d символов", i, MaxText)
	}
	return w, nil
}

// cellItem — одна запись ячейки (или диапазон MULBLANK/MULRK) вместе
// с записями, которые принадлежат формуле.
type cellItem struct {
	first, last int
	recs        []record
}

// patchSheet вписывает значения в подпоток листа и заново раскладывает
// таблицу ячеек: блоки ROW, ячейки, DBCELL.
func patchSheet(recs []record, writes []write) ([]record, error) {
	start, end := -1, -1
	for i, r := range recs {
		if r.id == recRow || isCell(r.id) || r.id == recDBCell || (start >= 0 && cellTail(r.id)) {
			if start < 0 {
				start = i
			}
			end = i
		}
	}
	if start < 0 {
		return nil, errors.New("в листе шаблона нет таблицы ячеек")
	}

	rows := map[int][]byte{}
	items := map[int][]cellItem{}
	for i := start; i <= end; i++ {
		r := recs[i]
		switch {
		case r.id == recRow:
			rows[int(le.Uint16(r.data))] = slices.Clone(r.data)
		case r.id == recDBCell:
		case isCell(r.id):
			row, col, _ := cellRef(r)
			last := col
			if r.id == recMulBlank || r.id == recMulRK {
				last = int(le.Uint16(r.data[len(r.data)-2:]))
			}
			items[row] = append(items[row], cellItem{first: col, last: last, recs: []record{r}})
		case cellTail(r.id):
			// Хвост формулы принадлежит последней прочитанной ячейке.
			prev, ok := lastCell(recs[start:i])
			if !ok {
				return nil, fmt.Errorf("запись 0x%04X без ячейки в таблице шаблона", r.id)
			}
			row, _, _ := cellRef(prev)
			l := items[row]
			l[len(l)-1].recs = append(l[len(l)-1].recs, r)
		default:
			return nil, fmt.Errorf("неожиданная запись 0x%04X в таблице ячеек шаблона", r.id)
		}
	}

	merges := mergedRanges(recs)
	for _, w := range writes {
		for _, m := range merges {
			if w.row >= m[0] && w.row <= m[1] && w.col >= m[2] && w.col <= m[3] &&
				(w.row != m[0] || w.col != m[2]) {
				return nil, fmt.Errorf("ячейка %d: (%d,%d) скрыта объединением ячеек шаблона", w.index, w.row, w.col)
			}
		}

		list := expandAt(items[w.row], w.col)
		pos := slices.IndexFunc(list, func(it cellItem) bool { return it.first <= w.col && w.col <= it.last })
		var xf int
		if pos >= 0 {
			if list[pos].recs[0].id == recFormula {
				return nil, fmt.Errorf("ячейка %d: (%d,%d) содержит формулу шаблона", w.index, w.row, w.col)
			}
			_, _, xf = cellRef(list[pos].recs[0])
		} else {
			xf = defaultXF(rows[w.row], recs, w.col)
			pos, _ = slices.BinarySearchFunc(list, w.col, func(it cellItem, col int) int { return it.first - col })
			list = slices.Insert(list, pos, cellItem{first: w.col, last: w.col})
		}
		list[pos].recs = []record{cellRecord(w, xf)}
		items[w.row] = list

		rowRec, ok := rows[w.row]
		if !ok {
			rowRec = make([]byte, 16)
			le.PutUint16(rowRec, u16(w.row))
			le.PutUint16(rowRec[2:], u16(w.col))
			le.PutUint16(rowRec[4:], u16(w.col+1))
			le.PutUint16(rowRec[6:], 0x00FF)
			le.PutUint32(rowRec[12:], 0x00000100)
			rows[w.row] = rowRec
		}
		le.PutUint16(rowRec[2:], u16(min(int(le.Uint16(rowRec[2:])), w.col)))
		le.PutUint16(rowRec[4:], u16(max(int(le.Uint16(rowRec[4:])), w.col+1)))
	}

	table := cellTable(rows, items)
	out := make([]record, 0, len(recs)+len(table))
	out = append(out, recs[:start]...)
	out = append(out, table...)
	out = append(out, recs[end+1:]...)
	growBounds(out, writes)
	return out, nil
}

func lastCell(recs []record) (record, bool) {
	for _, r := range slices.Backward(recs) {
		if isCell(r.id) {
			return r, true
		}
	}
	return record{}, false
}

// expandAt разворачивает MULBLANK/MULRK, покрывающий столбец, в отдельные
// BLANK/RK — так одну ячейку можно заменить, не трогая соседей.
func expandAt(list []cellItem, col int) []cellItem {
	pos := slices.IndexFunc(list, func(it cellItem) bool { return it.first <= col && col <= it.last })
	if pos < 0 || list[pos].first == list[pos].last {
		return list
	}
	r := list[pos].recs[0]
	row := r.data[:2]
	var singles []cellItem
	for c := list[pos].first; c <= list[pos].last; c++ {
		k := c - list[pos].first
		var rec record
		if r.id == recMulBlank {
			rec = record{id: recBlank, data: slices.Concat(row, le.AppendUint16(nil, u16(c)), r.data[4+2*k:6+2*k])}
		} else {
			rec = record{id: recRK, data: slices.Concat(row, le.AppendUint16(nil, u16(c)), r.data[4+6*k:10+6*k])}
		}
		singles = append(singles, cellItem{first: c, last: c, recs: []record{rec}})
	}
	return slices.Concat(list[:pos], singles, list[pos+1:])
}

// defaultXF — оформление пустой ячейки: из ROW, если у строки свой формат,
// иначе из COLINFO столбца.
func defaultXF(row []byte, recs []record, col int) int {
	if row != nil && le.Uint32(row[12:])&0x80 != 0 {
		return int(le.Uint32(row[12:])>>16) & 0x0FFF
	}
	for _, r := range recs {
		if r.id == recColInfo && len(r.data) >= 8 &&
			int(le.Uint16(r.data)) <= col && col <= int(le.Uint16(r.data[2:])) {
			return int(le.Uint16(r.data[6:]))
		}
	}
	return xfDefault
}

func cellRecord(w write, xf int) record {
	data := le.AppendUint16(nil, u16(w.row))
	data = le.AppendUint16(data, u16(w.col))
	data = le.AppendUint16(data, u16(xf))
	if w.number {
		return record{id: recNumber, data: le.AppendUint64(data, math.Float64bits(w.num))}
	}
	return record{id: recLabelSST, data: le.AppendUint32(data, u32(w.sst))}
}

// mergedRanges — объединения листа: первая и последняя строка, первый и
// последний столбец включительно.
func mergedRanges(recs []record) [][4]int {
	var out [][4]int
	for _, r := range recs {
		if r.id != recMergedCells || len(r.data) < 2 {
			continue
		}
		n := int(le.Uint16(r.data))
		for i := 0; i < n && 2+8*i+8 <= len(r.data); i++ {
			d := r.data[2+8*i:]
			out = append(out, [4]int{int(le.Uint16(d)), int(le.Uint16(d[2:])), int(le.Uint16(d[4:])), int(le.Uint16(d[6:]))})
		}
	}
	return out
}

// cellTable раскладывает строки блоками по 32, как Excel: записи ROW блока,
// затем его ячейки, затем DBCELL со смещениями до первой ячейки каждой строки.
func cellTable(rows map[int][]byte, items map[int][]cellItem) []record {
	for row, list := range items {
		if _, ok := rows[row]; !ok && len(list) > 0 {
			rec := make([]byte, 16)
			le.PutUint16(rec, u16(row))
			le.PutUint16(rec[2:], u16(list[0].first))
			le.PutUint16(rec[4:], u16(list[len(list)-1].last+1))
			le.PutUint16(rec[6:], 0x00FF)
			le.PutUint32(rec[12:], 0x00000100)
			rows[row] = rec
		}
	}
	order := make([]int, 0, len(rows))
	for row := range rows {
		order = append(order, row)
	}
	slices.Sort(order)

	var out []record
	for i := 0; i < len(order); {
		j := i
		for j < len(order) && order[j]/32 == order[i]/32 {
			j++
		}
		block := order[i:j]
		size := 0
		for _, row := range block {
			out = append(out, record{id: recRow, data: rows[row]})
			size += 4 + len(rows[row])
		}
		secondRow := 4 + len(rows[block[0]])
		prev := -1
		dbcell := le.AppendUint32(nil, 0)
		for _, row := range block {
			offset := 0
			if len(items[row]) > 0 {
				if prev < 0 {
					offset = size - secondRow
				} else {
					offset = size - prev
				}
				prev = size
			}
			dbcell = le.AppendUint16(dbcell, uint16(offset))
			for _, it := range items[row] {
				out = append(out, it.recs...)
				size += recordsSize(it.recs)
			}
		}
		le.PutUint32(dbcell, uint32(size))
		out = append(out, record{id: recDBCell, data: dbcell})
		i = j
	}
	return out
}

// growBounds расширяет DIMENSIONS и INDEX, если запись вышла за их пределы.
func growBounds(recs []record, writes []write) {
	for i, r := range recs {
		if r.id != recDimensions && r.id != recIndex {
			continue
		}
		d := slices.Clone(r.data)
		for _, w := range writes {
			if r.id == recDimensions && len(d) >= 12 {
				le.PutUint32(d, u32(min(int(le.Uint32(d)), w.row)))
				le.PutUint32(d[4:], u32(max(int(le.Uint32(d[4:])), w.row+1)))
				le.PutUint16(d[8:], u16(min(int(le.Uint16(d[8:])), w.col)))
				le.PutUint16(d[10:], u16(max(int(le.Uint16(d[10:])), w.col+1)))
			}
			if r.id == recIndex && len(d) >= 12 {
				le.PutUint32(d[4:], u32(min(int(le.Uint32(d[4:])), w.row)))
				le.PutUint32(d[8:], u32(max(int(le.Uint32(d[8:])), w.row+1)))
			}
		}
		recs[i].data = d
	}
}

// appendSST дописывает строки в конец SST, не трогая существующие байты.
// refs — сколько ссылок на строки добавилось в ячейках.
func appendSST(globals []record, add []string, refs int) ([]record, error) {
	first, last, ok := sstSpan(globals)
	if !ok {
		return nil, errors.New("в шаблоне нет таблицы строк SST")
	}
	chain := slices.Clone(globals[first : last+1])
	head := slices.Clone(chain[0].data)
	le.PutUint32(head, le.Uint32(head)+u32(refs))
	le.PutUint32(head[4:], le.Uint32(head[4:])+u32(len(add)))
	chain[0].data = head
	cur := slices.Clone(chain[len(chain)-1].data)

	flush := func(next []byte) {
		chain[len(chain)-1].data = cur
		chain = append(chain, record{id: recContinue})
		cur = next
	}
	for _, s := range add {
		units := utf16.Encode([]rune(s))
		need := 3
		if len(units) > 0 {
			need += 2
		}
		if len(cur)+need > maxRecord {
			flush(nil)
		}
		// Всегда UTF-16: флаг 0x01, без форматирования и фонетики.
		cur = le.AppendUint16(cur, u16(len(units)))
		cur = append(cur, 0x01)
		for len(units) > 0 {
			space := (maxRecord - len(cur)) / 2
			if space == 0 {
				// Продолжение символов начинается с байта флагов.
				flush([]byte{0x01})
				continue
			}
			n := min(space, len(units))
			for _, u := range units[:n] {
				cur = le.AppendUint16(cur, u)
			}
			units = units[n:]
		}
	}
	chain[len(chain)-1].data = cur
	return slices.Concat(globals[:first], chain, globals[last+1:]), nil
}

// bytes собирает поток Workbook и пересчитывает абсолютные смещения:
// EXTSST, BOUNDSHEET и INDEX каждого листа.
func (wb *workbook) bytes() ([]byte, error) {
	globals := slices.Clone(wb.globals)
	if err := rebuildExtSST(globals); err != nil {
		return nil, err
	}

	sheets := make([][]record, len(wb.sheets))
	pos := recordsSize(globals)
	starts := make([]int, len(sheets))
	for i, recs := range wb.sheets {
		recs = slices.Clone(recs)
		dbcells := 0
		for _, r := range recs {
			if r.id == recDBCell {
				dbcells++
			}
		}
		for j, r := range recs {
			if r.id == recIndex && len(r.data) >= 16 {
				d := make([]byte, 16+4*dbcells)
				copy(d, r.data[:16])
				recs[j].data = d
			}
		}
		sheets[i] = recs
		starts[i] = pos
		pos += recordsSize(recs)
	}

	k := 0
	for j, r := range globals {
		if r.id == recBoundSheet {
			d := slices.Clone(r.data)
			le.PutUint32(d, u32(starts[k]))
			globals[j].data = d
			k++
		}
	}

	out := make([]byte, 0, pos)
	for _, r := range globals {
		out = appendRecord(out, r)
	}
	for i, recs := range sheets {
		at := starts[i]
		index := -1
		var dbcells []int
		for _, r := range recs {
			switch r.id {
			case recIndex:
				index = at
			case recDBCell:
				dbcells = append(dbcells, at)
			}
			out = appendRecord(out, r)
			at += 4 + len(r.data)
		}
		if index < 0 {
			continue
		}
		body := out[index+4:]
		if def := findRecord(recs, recDefColWidth, starts[i]); def >= 0 {
			le.PutUint32(body[12:], u32(def))
		}
		for n, p := range dbcells {
			le.PutUint32(body[16+4*n:], u32(p))
		}
	}
	return out, nil
}

// findRecord возвращает абсолютное смещение первой записи id или -1.
func findRecord(recs []record, id uint16, at int) int {
	for _, r := range recs {
		if r.id == id {
			return at
		}
		at += 4 + len(r.data)
	}
	return -1
}

// rebuildExtSST пересчитывает EXTSST по фактическим позициям строк SST.
func rebuildExtSST(globals []record) error {
	ext := slices.IndexFunc(globals, func(r record) bool { return r.id == recExtSST })
	if ext < 0 {
		return nil
	}
	first, last, ok := sstSpan(globals)
	if !ok {
		return errors.New("в шаблоне есть EXTSST, но нет SST")
	}
	_, pos, err := parseSST(globals)
	if err != nil {
		return err
	}
	var recStart []int
	at := recordsSize(globals[:first])
	for _, r := range globals[first : last+1] {
		recStart = append(recStart, at)
		at += 4 + len(r.data)
	}

	dsst := max(8, len(pos)/128+1)
	data := le.AppendUint16(nil, u16(dsst))
	for i := 0; i < len(pos); i += dsst {
		p := pos[i]
		data = le.AppendUint32(data, u32(recStart[p.rec]+4+p.off))
		data = le.AppendUint16(data, u16(4+p.off))
		data = le.AppendUint16(data, 0)
	}
	if len(data) > maxRecord {
		return errors.New("таблица строк слишком велика для EXTSST")
	}
	globals[ext] = record{id: recExtSST, data: data}
	return nil
}

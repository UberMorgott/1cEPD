package xlsfill

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"unicode/utf16"
)

// Записи BIFF8, с которыми работает заполнение.
const (
	recBOF         = 0x0809
	recEOF         = 0x000A
	recBoundSheet  = 0x0085
	recSST         = 0x00FC
	recExtSST      = 0x00FF
	recContinue    = 0x003C
	recFilePass    = 0x002F
	recIndex       = 0x020B
	recDefColWidth = 0x0055
	recColInfo     = 0x007D
	recDimensions  = 0x0200
	recRow         = 0x0208
	recDBCell      = 0x00D7
	recMergedCells = 0x00E5
	recBlank       = 0x0201
	recMulBlank    = 0x00BE
	recNumber      = 0x0203
	recLabel       = 0x0204
	recBoolErr     = 0x0205
	recRK          = 0x027E
	recMulRK       = 0x00BD
	recLabelSST    = 0x00FD
	recRString     = 0x00D6
	recFormula     = 0x0006
	recString      = 0x0207
	recShrFmla     = 0x04BC
	recArray       = 0x0221
	recTable       = 0x0236
	recXF          = 0x00E0
	recFormat      = 0x041E

	maxRecord = 8224 // предел тела записи BIFF8
	xfDefault = 15   // XF ячейки по умолчанию
)

var le = binary.LittleEndian

type record struct {
	id   uint16
	data []byte
}

func parseRecords(b []byte) ([]record, error) {
	var out []record
	for pos := 0; pos < len(b); {
		if pos+4 > len(b) {
			return nil, errors.New("поток Workbook обрывается посреди записи")
		}
		n := int(le.Uint16(b[pos+2:]))
		if pos+4+n > len(b) {
			return nil, errors.New("поток Workbook обрывается посреди записи")
		}
		out = append(out, record{id: le.Uint16(b[pos:]), data: b[pos+4 : pos+4+n]})
		pos += 4 + n
	}
	return out, nil
}

func recordsSize(recs []record) int {
	n := 0
	for _, r := range recs {
		n += 4 + len(r.data)
	}
	return n
}

func appendRecord(out []byte, r record) []byte {
	out = le.AppendUint16(out, r.id)
	out = le.AppendUint16(out, u16(len(r.data)))
	return append(out, r.data...)
}

// workbook — поток Workbook, разложенный на подпотоки BOF…EOF.
type workbook struct {
	globals []record
	sheets  [][]record // подпотоки листов в порядке BOUNDSHEET
	strings []string   // таблица общих строк SST
}

func parseWorkbook(stream []byte) (*workbook, error) {
	recs, err := parseRecords(stream)
	if err != nil {
		return nil, err
	}
	// Подпотоки по смещению их BOF в исходном потоке.
	type sub struct {
		offset int
		recs   []record
	}
	var subs []sub
	pos, open := 0, -1
	for i, r := range recs {
		if r.id == recBOF && open < 0 {
			subs = append(subs, sub{offset: pos})
			open = i
		}
		if r.id == recFilePass {
			return nil, errors.New("шаблон зашифрован")
		}
		if r.id == recEOF && open >= 0 {
			subs[len(subs)-1].recs = recs[open : i+1]
			open = -1
		}
		pos += 4 + len(r.data)
	}
	if len(subs) == 0 || open >= 0 {
		return nil, errors.New("в потоке Workbook нет законченных подпотоков")
	}

	wb := &workbook{globals: subs[0].recs}
	for _, r := range wb.globals {
		if r.id != recBoundSheet {
			continue
		}
		if len(r.data) < 4 {
			return nil, errors.New("битая запись BOUNDSHEET")
		}
		at := int(le.Uint32(r.data))
		found := false
		for _, s := range subs[1:] {
			if s.offset == at {
				wb.sheets = append(wb.sheets, s.recs)
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("BOUNDSHEET указывает на смещение %d без листа", at)
		}
	}
	strs, _, err := parseSST(wb.globals)
	if err != nil {
		return nil, err
	}
	wb.strings = strs
	return wb, nil
}

// sstSpan — индексы записей SST и её CONTINUE в глобальном подпотоке.
func sstSpan(globals []record) (first, last int, ok bool) {
	for i, r := range globals {
		if r.id == recSST {
			last = i
			for last+1 < len(globals) && globals[last+1].id == recContinue {
				last++
			}
			return i, last, true
		}
	}
	return 0, 0, false
}

// sstPos — где начинается строка SST: номер записи в SST-цепочке и смещение
// внутри её тела.
type sstPos struct {
	rec, off int
}

// parseSST читает строки SST вместе с тем, где начинается каждая.
func parseSST(globals []record) ([]string, []sstPos, error) {
	first, last, ok := sstSpan(globals)
	if !ok {
		return nil, nil, nil
	}
	chain := globals[first : last+1]
	if len(chain[0].data) < 8 {
		return nil, nil, errors.New("битая запись SST")
	}
	unique := int(le.Uint32(chain[0].data[4:]))
	rec, off := 0, 8
	bad := errors.New("битая таблица строк SST")

	// skip пропускает n байт, переходя через границы CONTINUE.
	skip := func(n int) error {
		for n > 0 {
			if off == len(chain[rec].data) {
				rec++
				off = 0
				if rec >= len(chain) {
					return bad
				}
			}
			step := min(n, len(chain[rec].data)-off)
			off += step
			n -= step
		}
		return nil
	}

	strs := make([]string, 0, unique)
	pos := make([]sstPos, 0, unique)
	for range unique {
		if off == len(chain[rec].data) {
			rec++
			off = 0
		}
		if rec >= len(chain) || off+3 > len(chain[rec].data) {
			return nil, nil, bad
		}
		pos = append(pos, sstPos{rec, off})
		d := chain[rec].data
		cch := int(le.Uint16(d[off:]))
		flags := d[off+2]
		off += 3
		runs, ext := 0, 0
		if flags&0x08 != 0 {
			if off+2 > len(d) {
				return nil, nil, bad
			}
			runs = int(le.Uint16(d[off:]))
			off += 2
		}
		if flags&0x04 != 0 {
			if off+4 > len(d) {
				return nil, nil, bad
			}
			ext = int(le.Uint32(d[off:]))
			off += 4
		}
		wide := flags&0x01 != 0
		units := make([]uint16, 0, cch)
		for len(units) < cch {
			if off == len(chain[rec].data) {
				// Символы продолжаются в CONTINUE, которая начинается с байта флагов.
				rec++
				if rec >= len(chain) || len(chain[rec].data) == 0 {
					return nil, nil, bad
				}
				wide = chain[rec].data[0]&0x01 != 0
				off = 1
			}
			d = chain[rec].data
			if wide {
				if off+2 > len(d) {
					return nil, nil, bad
				}
				units = append(units, le.Uint16(d[off:]))
				off += 2
			} else {
				units = append(units, uint16(d[off]))
				off++
			}
		}
		if err := skip(4*runs + ext); err != nil {
			return nil, nil, err
		}
		strs = append(strs, string(utf16.Decode(units)))
	}
	return strs, pos, nil
}

// cellRef — адрес ячейки в записи BIFF8: строка, столбец, XF.
func cellRef(r record) (row, col, xf int) {
	return int(le.Uint16(r.data)), int(le.Uint16(r.data[2:])), int(le.Uint16(r.data[4:]))
}

// isCell — запись описывает ячейку (одну или диапазон MUL*).
func isCell(id uint16) bool {
	switch id {
	case recBlank, recNumber, recLabel, recBoolErr, recRK, recLabelSST, recRString,
		recFormula, recMulBlank, recMulRK:
		return true
	}
	return false
}

// cellTail — записи, которые идут сразу за формулой и принадлежат ей.
func cellTail(id uint16) bool {
	return id == recString || id == recShrFmla || id == recArray || id == recTable
}

// rkValue раскрывает число из формата RK.
func rkValue(rk uint32) float64 {
	var v float64
	if rk&0x02 != 0 {
		// 30-битное целое со знаком в старших битах.
		n := int64(rk >> 2)
		if rk&0x80000000 != 0 {
			n -= 1 << 30
		}
		v = float64(n)
	} else {
		v = math.Float64frombits(uint64(rk&0xFFFFFFFC) << 32)
	}
	if rk&0x01 != 0 {
		v /= 100
	}
	return v
}

// errLayout — поле BIFF8 или OLE2 не вмещает значение: такой файл не собрать.
type errLayout struct{ v int }

func (e errLayout) Error() string {
	return fmt.Sprintf("значение %d не помещается в поле .xls", e.v)
}

// u16 и u32 переводят номера и смещения в поля записи. Входные строки и
// столбцы проверены заранее, поэтому переполнение — сбой раскладки: паника
// с errLayout, которую Fill превращает в ошибку.
func u16(v int) uint16 {
	if v < 0 || v > math.MaxUint16 {
		panic(errLayout{v})
	}
	return uint16(v)
}

func u32(v int) uint32 {
	if v < 0 || v > math.MaxUint32 {
		panic(errLayout{v})
	}
	return uint32(v)
}

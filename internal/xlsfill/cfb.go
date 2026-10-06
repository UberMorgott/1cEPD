package xlsfill

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"slices"
	"unicode/utf16"
)

// Compound File Binary (OLE2) — контейнер, внутри которого .xls хранит поток
// Workbook. Читаем его целиком и собираем заново: поток Workbook после
// заполнения меняет длину, а остальные потоки и каталог переносятся как есть.

const (
	cfbSignature   = 0xE11AB1A1E011CFD0
	cfbHeaderSize  = 512
	cfbDirSize     = 128
	cfbFreeSect    = 0xFFFFFFFF
	cfbEndOfChain  = 0xFFFFFFFE
	cfbFATSect     = 0xFFFFFFFD
	cfbNoStream    = 0xFFFFFFFF
	cfbTypeStream  = 2
	cfbTypeRoot    = 5
	cfbHeaderDIFAT = 109
	// Запись ведём в версии 3: сектор 512 байт, мини-сектор 64.
	cfbSector     = 512
	cfbMiniSector = 64
	cfbMiniCutoff = 4096
)

// cfbFile — разобранный контейнер: сырые записи каталога и данные потоков.
type cfbFile struct {
	entries [][]byte       // записи каталога по 128 байт, в исходном порядке
	streams map[int][]byte // данные потоков по номеру записи каталога
	clsid   []byte         // CLSID из заголовка не используется, но переносится
}

func readCFB(data []byte) (*cfbFile, error) {
	if len(data) < cfbHeaderSize || binary.LittleEndian.Uint64(data) != cfbSignature {
		return nil, errors.New("не файл .xls: нет сигнатуры OLE2")
	}
	le := binary.LittleEndian
	sectorShift := le.Uint16(data[0x1E:])
	miniShift := le.Uint16(data[0x20:])
	if sectorShift != 9 && sectorShift != 12 || miniShift != 6 {
		return nil, fmt.Errorf("неподдерживаемый размер сектора OLE2: 2^%d", sectorShift)
	}
	sectorSize := 1 << sectorShift
	miniCutoff := le.Uint32(data[0x38:])

	sector := func(id uint32) ([]byte, error) {
		start := cfbHeaderSize + int(id)*sectorSize
		if sectorSize == 4096 {
			start = int(id+1) * sectorSize
		}
		if id >= cfbFATSect-1 || start+sectorSize > len(data) {
			return nil, fmt.Errorf("сектор %d вне файла", id)
		}
		return data[start : start+sectorSize], nil
	}

	// DIFAT: первые 109 номеров секторов FAT в заголовке, остальные — цепочкой.
	var fatSectors []uint32
	for i := range cfbHeaderDIFAT {
		if id := le.Uint32(data[0x4C+4*i:]); id != cfbFreeSect {
			fatSectors = append(fatSectors, id)
		}
	}
	next := le.Uint32(data[0x44:])
	for n := le.Uint32(data[0x48:]); n > 0 && next < cfbFATSect-1; n-- {
		sec, err := sector(next)
		if err != nil {
			return nil, err
		}
		for i := 0; i < sectorSize/4-1; i++ {
			if id := le.Uint32(sec[4*i:]); id != cfbFreeSect {
				fatSectors = append(fatSectors, id)
			}
		}
		next = le.Uint32(sec[sectorSize-4:])
	}
	var fat []uint32
	for _, id := range fatSectors {
		sec, err := sector(id)
		if err != nil {
			return nil, err
		}
		for i := 0; i < sectorSize/4; i++ {
			fat = append(fat, le.Uint32(sec[4*i:]))
		}
	}

	chain := func(start uint32) ([]byte, error) {
		var out []byte
		seen := map[uint32]bool{}
		for id := start; id != cfbEndOfChain; {
			if seen[id] || int(id) >= len(fat) {
				return nil, errors.New("битая цепочка секторов OLE2")
			}
			seen[id] = true
			sec, err := sector(id)
			if err != nil {
				return nil, err
			}
			out = append(out, sec...)
			id = fat[id]
		}
		return out, nil
	}

	dir, err := chain(le.Uint32(data[0x30:]))
	if err != nil {
		return nil, fmt.Errorf("каталог OLE2: %w", err)
	}
	f := &cfbFile{streams: map[int][]byte{}, clsid: append([]byte(nil), data[8:24]...)}
	for off := 0; off+cfbDirSize <= len(dir); off += cfbDirSize {
		f.entries = append(f.entries, append([]byte(nil), dir[off:off+cfbDirSize]...))
	}
	if len(f.entries) == 0 || f.entries[0][66] != cfbTypeRoot {
		return nil, errors.New("в OLE2 нет корневой записи")
	}

	root := f.entries[0]
	miniStream, err := chain(le.Uint32(root[116:]))
	if err != nil {
		return nil, fmt.Errorf("мини-поток OLE2: %w", err)
	}
	var miniFAT []uint32
	if first := le.Uint32(data[0x3C:]); first != cfbEndOfChain && first != cfbFreeSect {
		raw, err := chain(first)
		if err != nil {
			return nil, fmt.Errorf("mini FAT: %w", err)
		}
		for i := 0; i+4 <= len(raw); i += 4 {
			miniFAT = append(miniFAT, le.Uint32(raw[i:]))
		}
	}

	for i, e := range f.entries {
		if e[66] != cfbTypeStream {
			continue
		}
		size := int(le.Uint32(e[120:]))
		start := le.Uint32(e[116:])
		var raw []byte
		switch {
		case size == 0:
		case u32(size) < miniCutoff:
			seen := map[uint32]bool{}
			for id := start; id != cfbEndOfChain; id = miniFAT[id] {
				if seen[id] || int(id) >= len(miniFAT) || int(id+1)*cfbMiniSector > len(miniStream) {
					return nil, errors.New("битая цепочка мини-секторов OLE2")
				}
				seen[id] = true
				raw = append(raw, miniStream[int(id)*cfbMiniSector:int(id+1)*cfbMiniSector]...)
			}
		default:
			if raw, err = chain(start); err != nil {
				return nil, err
			}
		}
		if len(raw) < size {
			return nil, fmt.Errorf("поток OLE2 %q короче заявленного", entryName(e))
		}
		f.streams[i] = raw[:size]
	}
	return f, nil
}

func entryName(e []byte) string {
	n := int(binary.LittleEndian.Uint16(e[64:]))/2 - 1
	if n < 0 || n > 31 {
		return ""
	}
	u := make([]uint16, n)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(e[2*i:])
	}
	return string(utf16.Decode(u))
}

// stream находит поток верхнего уровня по имени.
func (f *cfbFile) stream(name string) (int, bool) {
	for i, e := range f.entries {
		if e[66] == cfbTypeStream && entryName(e) == name {
			return i, true
		}
	}
	return 0, false
}

// bytes собирает контейнер версии 3 заново: потоки ложатся подряд,
// записи каталога сохраняют дерево, меняются только старт и длина.
func (f *cfbFile) bytes() ([]byte, error) {
	le := binary.LittleEndian
	var body []byte // секторы начиная с номера 0
	var fat []uint32
	// place кладёт данные в новые секторы подряд и возвращает первый.
	place := func(data []byte, marker uint32) uint32 {
		first := u32(len(fat))
		n := (len(data) + cfbSector - 1) / cfbSector
		for i := range n {
			if marker != 0 {
				fat = append(fat, marker)
			} else if i == n-1 {
				fat = append(fat, cfbEndOfChain)
			} else {
				fat = append(fat, u32(len(fat))+1)
			}
		}
		padded := make([]byte, n*cfbSector)
		copy(padded, data)
		body = append(body, padded...)
		return first
	}

	entries := make([][]byte, len(f.entries))
	var mini []byte
	var miniFAT []uint32
	for i, src := range f.entries {
		e := append([]byte(nil), src...)
		entries[i] = e
		if e[66] != cfbTypeStream {
			continue
		}
		data := f.streams[i]
		le.PutUint64(e[120:], uint64(len(data)))
		switch {
		case len(data) == 0:
			le.PutUint32(e[116:], cfbEndOfChain)
		case len(data) < cfbMiniCutoff:
			first := u32(len(miniFAT))
			n := (len(data) + cfbMiniSector - 1) / cfbMiniSector
			for j := range n {
				if j == n-1 {
					miniFAT = append(miniFAT, cfbEndOfChain)
				} else {
					miniFAT = append(miniFAT, u32(len(miniFAT))+1)
				}
			}
			padded := make([]byte, n*cfbMiniSector)
			copy(padded, data)
			mini = append(mini, padded...)
			le.PutUint32(e[116:], first)
		default:
			le.PutUint32(e[116:], place(data, 0))
		}
	}

	root := entries[0]
	if len(mini) > 0 {
		le.PutUint32(root[116:], place(mini, 0))
	} else {
		le.PutUint32(root[116:], cfbEndOfChain)
	}
	le.PutUint64(root[120:], uint64(len(mini)))

	miniFATStart, miniFATCount := uint32(cfbEndOfChain), 0
	if len(miniFAT) > 0 {
		raw := make([]byte, 0, len(miniFAT)*4)
		for _, v := range miniFAT {
			raw = le.AppendUint32(raw, v)
		}
		for len(raw)%cfbSector != 0 {
			raw = le.AppendUint32(raw, cfbFreeSect)
		}
		miniFATCount = len(raw) / cfbSector
		miniFATStart = place(raw, 0)
	}

	dir := bytes.Join(entries, nil)
	for len(dir)%cfbSector != 0 {
		empty := make([]byte, cfbDirSize)
		le.PutUint32(empty[68:], cfbNoStream)
		le.PutUint32(empty[72:], cfbNoStream)
		le.PutUint32(empty[76:], cfbNoStream)
		dir = append(dir, empty...)
	}
	dirStart := place(dir, 0)

	// Секторы FAT описывают и сами себя: считаем, сколько их нужно.
	perSector := cfbSector / 4
	fatCount := 0
	for fatCount*perSector < len(fat)+fatCount {
		fatCount++
	}
	if fatCount > cfbHeaderDIFAT {
		return nil, errors.New("файл слишком велик для заголовка OLE2 без DIFAT")
	}
	fatStart := place(make([]byte, fatCount*cfbSector), cfbFATSect)
	for len(fat)%perSector != 0 {
		fat = append(fat, cfbFreeSect)
	}
	for i, v := range fat {
		le.PutUint32(body[int(fatStart)*cfbSector+4*i:], v)
	}

	header := make([]byte, cfbHeaderSize)
	le.PutUint64(header, cfbSignature)
	copy(header[8:], f.clsid)
	le.PutUint16(header[0x18:], 0x3E)
	le.PutUint16(header[0x1A:], 3)
	le.PutUint16(header[0x1C:], 0xFFFE)
	le.PutUint16(header[0x1E:], 9)
	le.PutUint16(header[0x20:], 6)
	le.PutUint32(header[0x2C:], uint32(fatCount))
	le.PutUint32(header[0x30:], dirStart)
	le.PutUint32(header[0x38:], cfbMiniCutoff)
	le.PutUint32(header[0x3C:], miniFATStart)
	le.PutUint32(header[0x40:], uint32(miniFATCount))
	le.PutUint32(header[0x44:], cfbEndOfChain)
	for i := range cfbHeaderDIFAT {
		v := uint32(cfbFreeSect)
		if i < fatCount {
			v = fatStart + uint32(i)
		}
		le.PutUint32(header[0x4C+4*i:], v)
	}
	return slices.Concat(header, body), nil
}

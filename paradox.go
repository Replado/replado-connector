package main

import (
	"encoding/binary"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func le16(b []byte, o int) int { return int(binary.LittleEndian.Uint16(b[o:])) }
func le32(b []byte, o int) int { return int(binary.LittleEndian.Uint32(b[o:])) }

type pxHeader struct {
	recSize, hdrSize, blockSize, numRecords, numFields, firstBlock int
	fileType, version                                              byte
	encrypted                                                      bool
}

func parsePxHeader(b []byte) (*pxHeader, bool) {
	if len(b) < 0x78 {
		return nil, false
	}
	h := &pxHeader{
		recSize: le16(b, 0), hdrSize: le16(b, 2), fileType: b[4],
		numRecords: le32(b, 6), firstBlock: le16(b, 0x0E),
		numFields: le16(b, 0x21), version: b[0x39],
	}
	max := int(b[5])
	h.blockSize = max * 1024
	switch max {
	case 1, 2, 4, 8, 16, 32:
	default:
		return nil, false
	}
	if h.fileType != 0 && h.fileType != 2 {
		return nil, false
	}
	if h.recSize == 0 || h.numFields == 0 || h.numFields > 255 || h.hdrSize < 0x58 || h.hdrSize%0x800 != 0 {
		return nil, false
	}
	if h.version < 3 || h.version > 0x0C {
		return nil, false
	}
	enc := binary.LittleEndian.Uint32(b[0x25:])
	if enc == 0xFF00FF00 {
		enc = 0
		if h.version >= 4 {
			enc = binary.LittleEndian.Uint32(b[0x5C:])
		}
	}
	h.encrypted = enc != 0
	return h, true
}

func sniffParadox(path string) (*pxHeader, bool) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	b := make([]byte, 0x80)
	if _, err := io.ReadFull(f, b); err != nil {
		return nil, false
	}
	return parsePxHeader(b)
}

func readParadox(path string) (*Table, error) {
	b, err := readFileShared(path)
	if err != nil {
		return nil, err
	}
	h, ok := parsePxHeader(b)
	if !ok {
		return nil, errf("keine Paradox-Tabelle: %s", path)
	}
	if h.encrypted {
		return nil, errf("Tabelle ist passwortgeschützt: %s", filepath.Base(path))
	}
	cp := 0
	off := 0x58
	if h.version >= 4 {
		off = 0x78
		cp = le16(b, 0x6A)
	}
	nf := h.numFields
	if off+2*nf > len(b) {
		return nil, errf("Kopf beschädigt")
	}
	types := make([]byte, nf)
	sizes := make([]int, nf)
	for i := 0; i < nf; i++ {
		types[i] = b[off+2*i]
		sizes[i] = int(b[off+2*i+1])
	}
	p := off + 2*nf + 4 + 4*nf
	if h.version >= 0x0C {
		p += 261
	} else {
		p += 79
	}
	names := make([]string, nf)
	for i := 0; i < nf; i++ {
		e := p
		for e < len(b) && b[e] != 0 {
			e++
		}
		if e >= len(b) {
			return nil, errf("Feldnamen beschädigt")
		}
		names[i] = string(b[p:e])
		p = e + 1
	}
	if _, ok := codepages[cp]; !ok {
		cp = guessCP(b[h.hdrSize:])
	}
	t := &Table{Name: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), Format: "paradox", Codepage: cp}
	for i := range names {
		t.Fields = append(t.Fields, Field{Name: decodeCP([]byte(names[i]), cp), Type: pxTypeName(types[i])})
	}

	seen := map[int]bool{}
	blk := h.firstBlock
	for blk != 0 && !seen[blk] {
		seen[blk] = true
		bo := h.hdrSize + (blk-1)*h.blockSize
		if bo+6 > len(b) {
			break
		}
		next := le16(b, bo)
		add := int(int16(binary.LittleEndian.Uint16(b[bo+4:])))
		if add >= 0 {
			n := add/h.recSize + 1
			for r := 0; r < n; r++ {
				ro := bo + 6 + r*h.recSize
				if ro+h.recSize > len(b) {
					break
				}
				row := make([]interface{}, nf)
				fo := ro
				for i := 0; i < nf; i++ {
					row[i] = pxValue(types[i], b[fo:fo+sizes[i]], cp)
					fo += sizes[i]
				}
				t.Rows = append(t.Rows, row)
			}
		}
		blk = next
	}
	return t, nil
}

func pxTypeName(t byte) string {
	switch t {
	case 0x01:
		return "text"
	case 0x02:
		return "date"
	case 0x03, 0x04, 0x16:
		return "integer"
	case 0x05:
		return "currency"
	case 0x06, 0x17:
		return "number"
	case 0x09:
		return "bool"
	case 0x0C, 0x0E:
		return "memo"
	case 0x14:
		return "time"
	case 0x15:
		return "timestamp"
	default:
		return "binary"
	}
}

func allZero(d []byte) bool {
	for _, c := range d {
		if c != 0 {
			return false
		}
	}
	return true
}

var day0 = time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)

func pxDouble(d []byte) float64 {
	x := make([]byte, 8)
	copy(x, d)
	if x[0]&0x80 != 0 {
		x[0] &= 0x7f
	} else {
		for i := range x {
			x[i] ^= 0xff
		}
	}
	return math.Float64frombits(binary.BigEndian.Uint64(x))
}

func pxValue(t byte, d []byte, cp int) interface{} {
	if t == 0x01 {
		e := 0
		for e < len(d) && d[e] != 0 {
			e++
		}
		return strings.TrimSpace(decodeCP(d[:e], cp))
	}
	if allZero(d) {
		return nil
	}
	switch t {
	case 0x02, 0x04, 0x16, 0x14:
		v := int64(int32(binary.BigEndian.Uint32(d) ^ 0x80000000))
		switch t {
		case 0x02:
			return day0.AddDate(0, 0, int(v)-1).Format("2006-01-02")
		case 0x14:
			return (time.Duration(v) * time.Millisecond).String()
		}
		return v
	case 0x03:
		return int64(int16(binary.BigEndian.Uint16(d) ^ 0x8000))
	case 0x05, 0x06:
		return round4(pxDouble(d))
	case 0x15:
		ms := pxDouble(d)
		return day0.Add(time.Duration(ms)*time.Millisecond - 24*time.Hour).Format("2006-01-02 15:04:05")
	case 0x09:
		return d[0]&0x7f != 0
	case 0x0C, 0x0E:
		if len(d) <= 10 {
			return nil
		}
		in := d[:len(d)-10]
		e := 0
		for e < len(in) && in[e] != 0 {
			e++
		}
		return strings.TrimSpace(decodeCP(in[:e], cp))
	}
	return nil
}

func round4(f float64) float64 { return math.Round(f*10000) / 10000 }

package main

import (
	"encoding/binary"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func readFileShared(path string) ([]byte, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if st.Size() > 1<<29 {
		return nil, errf("Datei zu groß für den Prototyp: %s", filepath.Base(path))
	}
	return ioutil.ReadFile(path)
}

type dbfHeader struct {
	version            byte
	numRecords, hdrLen int
	recLen, ldid       int
}

func parseDBFHeader(b []byte) (*dbfHeader, bool) {
	if len(b) < 32 {
		return nil, false
	}
	h := &dbfHeader{version: b[0], numRecords: le32(b, 4), hdrLen: le16(b, 8), recLen: le16(b, 10), ldid: int(b[29])}
	switch h.version {
	case 0x02, 0x03, 0x04, 0x05, 0x30, 0x31, 0x32, 0x43, 0x63, 0x83, 0x8B, 0x8E, 0xCB, 0xF5, 0xFB:
	default:
		return nil, false
	}
	mm, dd := int(b[2]), int(b[3])
	if mm < 1 || mm > 12 || dd < 1 || dd > 31 {
		return nil, false
	}
	if h.hdrLen < 33 || h.recLen < 2 {
		return nil, false
	}
	return h, true
}

func sniffDBF(path string) (*dbfHeader, bool) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	b := make([]byte, 32)
	if _, err := io.ReadFull(f, b); err != nil {
		return nil, false
	}
	return parseDBFHeader(b)
}

var ldidCP = map[int]int{0x01: 437, 0x02: 850, 0x03: 1252, 0x37: 850, 0x57: 1252, 0x58: 1252, 0x59: 1252, 0x64: 852, 0xC8: 1250, 0xC9: 1251}

type dbfField struct {
	name      string
	typ       byte
	size, dec int
}

func readDBF(path string) (*Table, error) {
	b, err := readFileShared(path)
	if err != nil {
		return nil, err
	}
	h, ok := parseDBFHeader(b)
	if !ok || h.hdrLen > len(b) {
		return nil, errf("keine DBF-Tabelle: %s", path)
	}
	var fs []dbfField
	for p := 32; p+32 <= h.hdrLen && b[p] != 0x0D; p += 32 {
		n := b[p : p+11]
		e := 0
		for e < len(n) && n[e] != 0 {
			e++
		}
		fs = append(fs, dbfField{name: string(n[:e]), typ: b[p+11], size: int(b[p+16]), dec: int(b[p+17])})
	}
	if len(fs) == 0 {
		return nil, errf("DBF ohne Felder")
	}
	cp, ok := ldidCP[h.ldid]
	if !ok {
		end := h.hdrLen + 64*1024
		if end > len(b) {
			end = len(b)
		}
		cp = guessCP(b[h.hdrLen:end])
	}
	t := &Table{Name: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), Format: "dbf", Codepage: cp}
	for _, f := range fs {
		t.Fields = append(t.Fields, Field{Name: f.name, Type: dbfTypeName(f.typ)})
	}
	for r := 0; r < h.numRecords; r++ {
		ro := h.hdrLen + r*h.recLen
		if ro+h.recLen > len(b) {
			break
		}
		if b[ro] == '*' {
			continue
		}
		row := make([]interface{}, len(fs))
		fo := ro + 1
		for i, f := range fs {
			if fo+f.size > ro+h.recLen {
				break
			}
			row[i] = dbfValue(f, b[fo:fo+f.size], cp)
			fo += f.size
		}
		t.Rows = append(t.Rows, row)
	}
	return t, nil
}

func dbfTypeName(t byte) string {
	switch t {
	case 'C', 'V':
		return "text"
	case 'N', 'F', 'B', 'Y', 'O':
		return "number"
	case 'I', '+':
		return "integer"
	case 'D':
		return "date"
	case 'T', '@':
		return "timestamp"
	case 'L':
		return "bool"
	case 'M', 'G', 'P':
		return "memo"
	}
	return "binary"
}

func dbfValue(f dbfField, d []byte, cp int) interface{} {
	switch f.typ {
	case 'C', 'V':
		return strings.TrimSpace(decodeCP(trimNul(d), cp))
	case 'N', 'F':
		s := strings.TrimSpace(string(trimNul(d)))
		if s == "" || strings.Trim(s, "*") == "" {
			return nil
		}
		v, err := strconv.ParseFloat(strings.Replace(s, ",", ".", -1), 64)
		if err != nil {
			return nil
		}
		if f.dec == 0 && v == math.Trunc(v) {
			return int64(v)
		}
		return v
	case 'D':
		s := string(d)
		if len(s) != 8 || strings.TrimSpace(s) == "" || s == "00000000" {
			return nil
		}
		return s[0:4] + "-" + s[4:6] + "-" + s[6:8]
	case 'L':
		switch d[0] {
		case 'T', 't', 'Y', 'y', 'J', 'j':
			return true
		case 'F', 'f', 'N', 'n':
			return false
		}
		return nil
	case 'I', '+':
		if len(d) == 4 {
			return int64(int32(binary.LittleEndian.Uint32(d)))
		}
	case 'Y':
		if len(d) == 8 {
			return float64(int64(binary.LittleEndian.Uint64(d))) / 10000
		}
	case 'B', 'O':
		if len(d) == 8 {
			return round4(math.Float64frombits(binary.LittleEndian.Uint64(d)))
		}
	case 'T', '@':
		if len(d) == 8 {
			jd := int(int32(binary.LittleEndian.Uint32(d)))
			ms := int(int32(binary.LittleEndian.Uint32(d[4:])))
			if jd == 0 {
				return nil
			}
			l := jd + 68569
			n := 4 * l / 146097
			l = l - (146097*n+3)/4
			i := 4000 * (l + 1) / 1461001
			l = l - 1461*i/4 + 31
			j := 80 * l / 2447
			day := l - 2447*j/80
			l = j / 11
			month := j + 2 - 12*l
			year := 100*(n-49) + i + l
			return pad(year, 4) + "-" + pad(month, 2) + "-" + pad(day, 2) + " " + pad(ms/3600000, 2) + ":" + pad(ms/60000%60, 2) + ":" + pad(ms/1000%60, 2)
		}
	}
	return nil
}

func trimNul(d []byte) []byte {
	for i, c := range d {
		if c == 0 {
			return d[:i]
		}
	}
	return d
}

func pad(v, n int) string {
	s := strconv.Itoa(v)
	for len(s) < n {
		s = "0" + s
	}
	return s
}

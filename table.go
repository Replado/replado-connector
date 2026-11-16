package main

import (
	"os"
	"strings"
	"sync"
	"time"
)

type Field struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type Table struct {
	Name     string          `json:"name"`
	Format   string          `json:"format"`
	Codepage int             `json:"codepage,omitempty"`
	Fields   []Field         `json:"fields"`
	Rows     [][]interface{} `json:"-"`
}

func (t *Table) FieldIndex(name string) int {
	for i, f := range t.Fields {
		if strings.EqualFold(f.Name, name) {
			return i
		}
	}
	return -1
}

func (t *Table) RowMap(r []interface{}, only []int) map[string]interface{} {
	m := make(map[string]interface{})
	if only != nil {
		for _, i := range only {
			m[t.Fields[i].Name] = r[i]
		}
		return m
	}
	for i, f := range t.Fields {
		if r[i] == nil || r[i] == "" {
			continue
		}
		m[f.Name] = r[i]
	}
	return m
}

type cacheEntry struct {
	mod  time.Time
	size int64
	t    *Table
}

var (
	tcache   = map[string]cacheEntry{}
	tcacheMu sync.Mutex
)

func LoadTable(path string) (*Table, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	tcacheMu.Lock()
	if c, ok := tcache[path]; ok && c.mod.Equal(st.ModTime()) && c.size == st.Size() {
		tcacheMu.Unlock()
		return c.t, nil
	}
	tcacheMu.Unlock()

	var t *Table
	switch strings.ToLower(extOf(path)) {
	case ".db":
		t, err = readParadox(path)
	case ".dbf":
		t, err = readDBF(path)
	default:
		err = errf("unbekanntes Format: %s", path)
	}
	if err != nil {
		return nil, err
	}
	tcacheMu.Lock()
	if len(tcache) > 64 {
		tcache = map[string]cacheEntry{}
	}
	tcache[path] = cacheEntry{st.ModTime(), st.Size(), t}
	tcacheMu.Unlock()
	return t, nil
}

func decodeCP(b []byte, cp int) string {
	tab, ok := codepages[cp]
	if !ok {
		tab = codepages[1252]
	}
	var sb strings.Builder
	for _, c := range b {
		if c < 0x80 {
			sb.WriteByte(c)
		} else {
			sb.WriteRune(tab[c-0x80])
		}
	}
	return strings.TrimRight(sb.String(), " ")
}

func guessCP(b []byte) int {
	dos, win := 0, 0
	for _, c := range b {
		switch c {
		case 0x81, 0x84, 0x94, 0x8E, 0x99, 0x9A, 0xE1:
			dos++
		case 0xE4, 0xF6, 0xFC, 0xC4, 0xD6, 0xDC, 0xDF:
			win++
		}
	}
	if win > dos {
		return 1252
	}
	return 850
}

func extOf(p string) string {
	i := strings.LastIndex(p, ".")
	if i < 0 || strings.ContainsAny(p[i:], `/\`) {
		return ""
	}
	return p[i:]
}

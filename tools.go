package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

type ToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

var toolDefs = []ToolDef{
	{"list_sources", "Listet die lokalen Datenbestände (z.B. einer alten Faktura-Software) auf diesem PC: ID, Pfad, erkannte Software, Hinweise zur Bedeutung der Tabellen und Tabellen mit Zeilenzahl. Immer zuerst aufrufen.",
		json.RawMessage(`{"type":"object","properties":{"rescan":{"type":"boolean","description":"Festplatte neu durchsuchen"}}}`)},
	{"describe_table", "Zeigt Spalten (Name, Typ) und einige Beispielzeilen einer Tabelle. Nutzen, um die Bedeutung einer unbekannten Tabelle zu verstehen.",
		json.RawMessage(`{"type":"object","required":["source","table"],"properties":{"source":{"type":"string"},"table":{"type":"string"},"sample":{"type":"integer","description":"Anzahl Beispielzeilen, Standard 5"}}}`)},
	{"query", "Liest Zeilen aus einer Tabelle mit Filtern, Sortierung und Limit. Operatoren: eq, ne, contains, gt, gte, lt, lte. Datumswerte als JJJJ-MM-TT.",
		json.RawMessage(`{"type":"object","required":["source","table"],"properties":{"source":{"type":"string"},"table":{"type":"string"},"where":{"type":"array","items":{"type":"object","required":["field","op","value"],"properties":{"field":{"type":"string"},"op":{"type":"string","enum":["eq","ne","contains","gt","gte","lt","lte"]},"value":{}}}},"order_by":{"type":"string"},"desc":{"type":"boolean"},"fields":{"type":"array","items":{"type":"string"}},"limit":{"type":"integer","description":"max. 200, Standard 50"}}}`)},
	{"search", "Volltextsuche über alle Textspalten einer Quelle oder einer Tabelle. Jedes Wort des Suchtexts muss irgendwo in der Zeile vorkommen, auch verteilt auf mehrere Spalten; Groß/Klein, Umlaute (ö = oe, ß = ss), Bindestriche und Leerzeichen spielen keine Rolle. Gut, um z.B. einen Kunden anhand von Firma, Name, Ort oder E-Mail zu finden.",
		json.RawMessage(`{"type":"object","required":["source","text"],"properties":{"source":{"type":"string"},"text":{"type":"string"},"table":{"type":"string"},"limit":{"type":"integer","description":"max. Treffer, Standard 20"}}}`)},
}

func callTool(name string, raw json.RawMessage) (interface{}, error) {
	var a struct {
		Rescan  bool     `json:"rescan"`
		Source  string   `json:"source"`
		Table   string   `json:"table"`
		Sample  int      `json:"sample"`
		Text    string   `json:"text"`
		OrderBy string   `json:"order_by"`
		Desc    bool     `json:"desc"`
		Fields  []string `json:"fields"`
		Limit   int      `json:"limit"`
		Where   []struct {
			Field string      `json:"field"`
			Op    string      `json:"op"`
			Value interface{} `json:"value"`
		} `json:"where"`
	}
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, errf("ungültige Argumente: %v", err)
		}
	}
	switch name {
	case "list_sources":
		if a.Rescan || getSources() == nil {
			Scan(scanRoots())
		}
		return map[string]interface{}{"sources": getSources()}, nil

	case "describe_table":
		t, err := openTable(a.Source, a.Table)
		if err != nil {
			return nil, err
		}
		n := a.Sample
		if n <= 0 {
			n = 5
		}
		if n > 50 {
			n = 50
		}
		var sample []map[string]interface{}
		for i := len(t.Rows) - 1; i >= 0 && len(sample) < n; i-- {
			sample = append(sample, t.RowMap(t.Rows[i], nil))
		}
		return map[string]interface{}{"table": t.Name, "format": t.Format, "rows": len(t.Rows), "fields": t.Fields, "sample": sample}, nil

	case "query":
		t, err := openTable(a.Source, a.Table)
		if err != nil {
			return nil, err
		}
		type cond struct {
			idx int
			op  string
			val interface{}
		}
		var cs []cond
		for _, w := range a.Where {
			i := t.FieldIndex(w.Field)
			if i < 0 {
				return nil, errf("Spalte %q gibt es in %s nicht", w.Field, t.Name)
			}
			cs = append(cs, cond{i, strings.ToLower(w.Op), w.Value})
		}
		var hits [][]interface{}
		for _, r := range t.Rows {
			ok := true
			for _, c := range cs {
				if !match(r[c.idx], c.op, c.val) {
					ok = false
					break
				}
			}
			if ok {
				hits = append(hits, r)
			}
		}
		if a.OrderBy != "" {
			i := t.FieldIndex(a.OrderBy)
			if i < 0 {
				return nil, errf("Spalte %q gibt es in %s nicht", a.OrderBy, t.Name)
			}
			sort.SliceStable(hits, func(x, y int) bool {
				c := compare(hits[x][i], hits[y][i])
				if a.Desc {
					return c > 0
				}
				return c < 0
			})
		}
		var only []int
		for _, f := range a.Fields {
			i := t.FieldIndex(f)
			if i < 0 {
				return nil, errf("Spalte %q gibt es in %s nicht", f, t.Name)
			}
			only = append(only, i)
		}
		lim := clampLimit(a.Limit, 50, 200)
		var rows []map[string]interface{}
		for i := 0; i < len(hits) && i < lim; i++ {
			rows = append(rows, t.RowMap(hits[i], only))
		}
		return map[string]interface{}{"table": t.Name, "matches": len(hits), "returned": len(rows), "rows": rows}, nil

	case "search":
		s := findSource(a.Source)
		if s == nil {
			return nil, errf("Quelle %q unbekannt, erst list_sources aufrufen", a.Source)
		}
		words := searchWords(a.Text)
		if len(words) == 0 {
			return nil, errf("Suchtext fehlt")
		}
		lim := clampLimit(a.Limit, 20, 100)
		var out []map[string]interface{}
		for _, ti := range s.Tables {
			if a.Table != "" && !strings.EqualFold(a.Table, ti.Name) {
				continue
			}
			if ti.Locked || len(out) >= lim {
				continue
			}
			t, err := LoadTable(ti.file)
			if err != nil {
				continue
			}
			for _, r := range t.Rows {
				if rowMatches(r, words) {
					out = append(out, map[string]interface{}{"table": t.Name, "row": t.RowMap(r, nil)})
				}
				if len(out) >= lim {
					break
				}
			}
		}
		return map[string]interface{}{"hits": out}, nil
	}
	return nil, errf("unbekanntes Werkzeug %q", name)
}

var umlauts = strings.NewReplacer("ä", "ae", "ö", "oe", "ü", "ue", "ß", "ss")

func normalized(s string) string {
	s = umlauts.Replace(strings.ToLower(s))
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return -1
	}, s)
}

func searchWords(text string) []string {
	var out []string
	for _, w := range strings.Fields(text) {
		if n := normalized(w); n != "" {
			out = append(out, n)
		}
	}
	return out
}

func rowMatches(r []interface{}, words []string) bool {
	var b strings.Builder
	for _, v := range r {
		if sv, ok := v.(string); ok {
			b.WriteString(normalized(sv))
			b.WriteByte(' ')
		}
	}
	text := b.String()
	for _, w := range words {
		if !strings.Contains(text, w) {
			return false
		}
	}
	return true
}

func openTable(src, table string) (*Table, error) {
	if getSources() == nil {
		Scan(scanRoots())
	}
	s := findSource(src)
	if s == nil {
		return nil, errf("Quelle %q unbekannt, erst list_sources aufrufen", src)
	}
	f := s.tableFile(table)
	if f == "" {
		return nil, errf("Tabelle %q gibt es in Quelle %s nicht", table, s.ID)
	}
	return LoadTable(f)
}

func clampLimit(v, def, max int) int {
	if v <= 0 {
		return def
	}
	if v > max {
		return max
	}
	return v
}

func toFloat(v interface{}) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int64:
		return float64(x), true
	case int:
		return float64(x), true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.Replace(strings.TrimSpace(x), ",", ".", -1), 64)
		return f, err == nil
	}
	return 0, false
}

func toStr(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

func compare(a, b interface{}) int {
	if a == nil || b == nil {
		switch {
		case a == nil && b == nil:
			return 0
		case a == nil:
			return -1
		}
		return 1
	}
	as, aStr := a.(string)
	bs, bStr := b.(string)
	numericText := aStr && bStr && isNum(as) && isNum(bs)
	if !aStr || !bStr || numericText {
		fa, ok1 := toFloat(a)
		fb, ok2 := toFloat(b)
		if ok1 && ok2 {
			switch {
			case fa < fb:
				return -1
			case fa > fb:
				return 1
			}
			return 0
		}
	}
	return strings.Compare(strings.ToLower(toStr(a)), strings.ToLower(toStr(b)))
}

func match(v interface{}, op string, want interface{}) bool {
	switch op {
	case "contains":
		return strings.Contains(strings.ToLower(toStr(v)), strings.ToLower(toStr(want)))
	case "eq":
		if want == nil {
			return v == nil || v == ""
		}
		return compare(v, want) == 0 || strings.EqualFold(strings.TrimSpace(toStr(v)), strings.TrimSpace(toStr(want)))
	case "ne":
		return !match(v, "eq", want)
	}
	if v == nil {
		return false
	}
	c := compare(v, want)
	switch op {
	case "gt":
		return c > 0
	case "gte":
		return c >= 0
	case "lt":
		return c < 0
	case "lte":
		return c <= 0
	}
	return false
}

func isNum(s string) bool {
	if s == "" || strings.Count(s, "-") > 1 {
		return false
	}
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

func errf(f string, a ...interface{}) error { return fmt.Errorf(f, a...) }

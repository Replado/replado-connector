package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func fillProtocol() {
	record(Entry{Kind: "anmeldung", Text: "Angemeldet"})
	record(Entry{Kind: "ordner", Text: "Ordner freigegeben"})
	for i := 0; i < 30; i++ {
		record(Entry{Kind: "abfrage", Tool: "query", Table: "Kunden" + strconv.Itoa(i), Rows: i})
	}
	record(Entry{Kind: "abfrage", Tool: "search", Table: "Artikel", Error: "Tabelle gesperrt"})
}

func TestProtocolFilterSortsEntries(t *testing.T) {
	defer useAppDir(t)()
	fillProtocol()
	cases := map[string]int{"": 33, "abfragen": 30, "fehler": 1, "einrichtung": 2, "unbekannt": 33}
	for filter, want := range cases {
		got, more := readProtocolPage(filter, 100)
		if len(got) != want || more {
			t.Errorf("Filter %q: %d Einträge, mehr=%v, erwartet %d", filter, len(got), more, want)
		}
	}
	errors, _ := readProtocolPage("fehler", 10)
	if errors[0].Error != "Tabelle gesperrt" {
		t.Errorf("Fehlerfilter liefert %+v", errors[0])
	}
}

func TestProtocolPageNewestFirstWithMore(t *testing.T) {
	defer useAppDir(t)()
	fillProtocol()
	page, more := readProtocolPage("abfragen", 25)
	if len(page) != 25 || !more {
		t.Fatalf("Seite: %d Einträge, mehr=%v", len(page), more)
	}
	if page[0].Table != "Kunden29" {
		t.Errorf("Neuester Eintrag zuerst erwartet, bekam %q", page[0].Table)
	}
	rest, more := readProtocolPage("abfragen", 30)
	if len(rest) != 30 || more {
		t.Errorf("Zweite Seite: %d Einträge, mehr=%v", len(rest), more)
	}
}

func callUI(u *uiServer, path string, body interface{}) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", path, bytes.NewReader(b))
	req.Host = strings.TrimPrefix(u.origin, "http://")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Replado-Token", u.token)
	rec := httptest.NewRecorder()
	u.ServeHTTP(rec, req)
	return rec
}

func TestUILogActionFiltersAndPages(t *testing.T) {
	defer useAppDir(t)()
	fillProtocol()
	u := &uiServer{token: newToken(), origin: "http://127.0.0.1:1234"}
	rec := callUI(u, "/api/log", map[string]interface{}{"filter": "fehler"})
	var res struct {
		Entries []Entry `json:"entries"`
		More    bool    `json:"more"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("Antwort %d: %s", rec.Code, rec.Body.String())
	}
	if len(res.Entries) != 1 || res.More {
		t.Errorf("Fehlerfilter: %+v", res)
	}
	rec = callUI(u, "/api/log", map[string]interface{}{})
	json.Unmarshal(rec.Body.Bytes(), &res)
	if len(res.Entries) != 25 || !res.More {
		t.Errorf("Standardseite: %d Einträge, mehr=%v", len(res.Entries), res.More)
	}
}

func TestUIPageHasTabsAndKeepsTheSetupApart(t *testing.T) {
	u := &uiServer{token: newToken(), origin: "http://127.0.0.1:1234"}
	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	u.ServeHTTP(rec, req)
	page := rec.Body.String()
	for _, want := range []string{`role="tablist"`, `id="tab-overview"`, `id="tab-folders"`, `id="tab-log"`, `id="log-filter"`, `id="more-log"`, `id="setup"`} {
		if !strings.Contains(page, want) {
			t.Errorf("Seite enthält %s nicht", want)
		}
	}
	if strings.Contains(page, "1. Mit Replado verbinden") {
		t.Error("Die alten nummerierten Schritte sind noch da")
	}
}

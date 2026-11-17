package main

import (
	"encoding/binary"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func tempDir(t *testing.T) string {
	dir, err := ioutil.TempDir("", "replado-connector-test")
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func useAppDir(t *testing.T) func() {
	dir := tempDir(t)
	old := os.Getenv("REPLADO_CONNECTOR_DIR")
	os.Setenv("REPLADO_CONNECTOR_DIR", dir)
	return func() {
		os.Setenv("REPLADO_CONNECTOR_DIR", old)
		os.RemoveAll(dir)
	}
}

func writeDBF(t *testing.T, path string, fields []string, size int, rows [][]string) {
	hdrLen := 32 + 32*len(fields) + 1
	recLen := 1 + size*len(fields)
	b := make([]byte, hdrLen)
	b[0] = 0x03
	b[1], b[2], b[3] = 126, 11, 10
	binary.LittleEndian.PutUint32(b[4:], uint32(len(rows)))
	binary.LittleEndian.PutUint16(b[8:], uint16(hdrLen))
	binary.LittleEndian.PutUint16(b[10:], uint16(recLen))
	b[29] = 0x57
	for i, f := range fields {
		p := 32 + 32*i
		copy(b[p:p+11], f)
		b[p+11] = 'C'
		b[p+16] = byte(size)
	}
	b[hdrLen-1] = 0x0D
	for _, row := range rows {
		rec := make([]byte, recLen)
		rec[0] = ' '
		for i := range fields {
			cell := []byte(strings.Repeat(" ", size))
			copy(cell, row[i])
			copy(rec[1+size*i:], cell)
		}
		b = append(b, rec...)
	}
	b = append(b, 0x1A)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
}

func sampleData(t *testing.T) (string, string) {
	base := tempDir(t)
	allowed := filepath.Join(base, "Firma", "Daten")
	other := filepath.Join(base, "Privat")
	writeDBF(t, filepath.Join(allowed, "KUNDEN.DBF"), []string{"NR", "NAME", "EMAIL"}, 30, [][]string{
		{"4711", "Hans Meier", "hans.meier@fc-muster.de"},
		{"4712", "Anna Schmidt", "anna@beispiel.de"},
	})
	writeDBF(t, filepath.Join(other, "GEHEIM.DBF"), []string{"NAME"}, 20, [][]string{{"Nur privat"}})
	return allowed, other
}

func TestNormalizeServer(t *testing.T) {
	ok := map[string]string{
		"https://replado.firma.de":      "https://replado.firma.de",
		"replado.firma.de/":             "https://replado.firma.de",
		"https://firma.de/replado/api/": "https://firma.de/replado",
		"http://192.168.1.20:8000":      "http://192.168.1.20:8000",
		"http://localhost:8000":         "http://localhost:8000",
		"http://10.0.0.5":               "http://10.0.0.5",
	}
	for in, want := range ok {
		got, err := normalizeServer(in)
		if err != nil || got != want {
			t.Errorf("normalizeServer(%q) = %q, %v; erwartet %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "http://replado.firma.de", "http://8.8.8.8", "ftp://firma.de", "javascript:alert(1)", "https://nutzer:pw@firma.de", "https://firma.de/?x=1", "http://172.32.0.1"} {
		if got, err := normalizeServer(in); err == nil {
			t.Errorf("normalizeServer(%q) = %q, erwartet Fehler", in, got)
		}
	}
}

func TestFolderMustBeConcreteAndExisting(t *testing.T) {
	allowed, _ := sampleData(t)
	if _, err := validateFolder(allowed); err != nil {
		t.Fatalf("Ordner abgelehnt: %v", err)
	}
	rejected := []string{"", string(os.PathSeparator), homeDir(), filepath.Join(allowed, "gibt-es-nicht"), filepath.Join(allowed, "KUNDEN.DBF")}
	for _, d := range systemDirs() {
		rejected = append(rejected, filepath.Join(d, "unter"))
	}
	for _, p := range rejected {
		if _, err := validateFolder(p); err == nil {
			t.Errorf("validateFolder(%q) sollte abgelehnt werden", p)
		}
	}
}

func TestInsideDoesNotMatchNeighbourFolders(t *testing.T) {
	sep := string(os.PathSeparator)
	base := sep + "daten" + sep + "topfakt"
	cases := map[string]bool{
		base:                          true,
		base + sep + "2025":           true,
		strings.ToUpper(base):         true,
		base + "-alt":                 false,
		sep + "daten":                 false,
		base + sep + ".." + sep + "x": false,
	}
	for p, want := range cases {
		if got := inside(p, base); got != want {
			t.Errorf("inside(%q) = %v, erwartet %v", p, got, want)
		}
	}
}

func TestAddFolderNeedsConfirmationAndStoresText(t *testing.T) {
	defer useAppDir(t)()
	allowed, _ := sampleData(t)
	c := loadConfig()
	if err := addFolder(c, allowed, false); err == nil {
		t.Fatal("ohne Bestätigung freigegeben")
	}
	if err := addFolder(c, allowed, true); err != nil {
		t.Fatal(err)
	}
	if err := addFolder(c, allowed+string(os.PathSeparator), true); err != nil {
		t.Fatal(err)
	}
	if len(c.Folders) != 1 || c.Folders[0].Confirmation != confirmationText || c.Folders[0].ConfirmedAt == "" {
		t.Fatalf("Ordner falsch gespeichert: %+v", c.Folders)
	}
	if !removeFolder(c, allowed) || len(c.Folders) != 0 {
		t.Fatal("Ordner nicht entfernt")
	}
}

func TestToolsReadOnlyConfirmedFolders(t *testing.T) {
	allowed, other := sampleData(t)
	found := Scan([]string{allowed, other})
	if len(found) != 2 {
		t.Fatalf("erwartet 2 Quellen, gefunden %d", len(found))
	}
	setRoots([]string{allowed})
	defer setRoots(nil)
	var allowedID, otherID string
	for _, s := range found {
		if strings.HasPrefix(s.Path, allowed) {
			allowedID = s.ID
		} else {
			otherID = s.ID
		}
	}
	res, err := callTool("query", json.RawMessage(`{"source":"`+allowedID+`","table":"kunden","where":[{"field":"NR","op":"eq","value":"4711"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if rowsOf(res) != 1 {
		t.Fatalf("erwartet eine Zeile: %+v", res)
	}
	if _, err := callTool("query", json.RawMessage(`{"source":"`+otherID+`","table":"GEHEIM"}`)); err == nil {
		t.Fatal("Quelle außerhalb der freigegebenen Ordner wurde gelesen")
	}
	if _, err := callTool("search", json.RawMessage(`{"source":"`+otherID+`","text":"privat"}`)); err == nil {
		t.Fatal("Suche außerhalb der freigegebenen Ordner")
	}
	if _, err := callTool("query", json.RawMessage(`{"source":"`+allowedID+`","table":"../Privat/GEHEIM"}`)); err == nil {
		t.Fatal("Tabellenname mit Pfad wurde akzeptiert")
	}
}

func newUI(t *testing.T) (*uiServer, *httptest.Server) {
	u := &uiServer{token: newToken(), client: httpClient(5 * time.Second)}
	u.touch()
	srv := httptest.NewServer(u)
	u.origin = srv.URL
	return u, srv
}

func uiRequestTo(t *testing.T, srv *httptest.Server, method, path, token, origin, contentType, body string) int {
	req, _ := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("X-Replado-Token", token)
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func TestLocalPageRejectsForeignRequests(t *testing.T) {
	defer useAppDir(t)()
	u, srv := newUI(t)
	defer srv.Close()
	j := "application/json"
	checks := []struct {
		name                        string
		method, path, token, origin string
		contentType                 string
		want                        int
	}{
		{"Seite ohne Schlüssel", "GET", "/", "", "", "", 200},
		{"API ohne Schlüssel", "POST", "/api/state", "", "", j, 401},
		{"API mit falschem Schlüssel", "POST", "/api/state", strings.Repeat("0", 64), "", j, 401},
		{"fremde Webseite", "POST", "/api/state", u.token, "https://boese.example", j, 403},
		{"GET auf API", "GET", "/api/state", u.token, "", "", 405},
		{"Formular statt JSON", "POST", "/api/state", u.token, "", "text/plain", 415},
		{"richtig", "POST", "/api/state", u.token, srv.URL, j, 200},
	}
	for _, c := range checks {
		if got := uiRequestTo(t, srv, c.method, c.path, c.token, c.origin, c.contentType, "{}"); got != c.want {
			t.Errorf("%s: Status %d, erwartet %d", c.name, got, c.want)
		}
	}
	req, _ := http.NewRequest("POST", srv.URL+"/api/state", strings.NewReader("{}"))
	req.Host = "boese.example"
	req.Header.Set("X-Replado-Token", u.token)
	req.Header.Set("Content-Type", j)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Errorf("fremder Host (DNS-Rebinding): Status %d", resp.StatusCode)
	}
	if uiRequestTo(t, srv, "POST", "/api/state", u.token, "", j, strings.Repeat("x", maxUIBody+10)) != 413 {
		t.Error("zu große Anfrage angenommen")
	}
}

func TestLocalPageHasStrictHeaders(t *testing.T) {
	_, srv := newUI(t)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if !strings.Contains(resp.Header.Get("Content-Security-Policy"), "frame-ancestors 'none'") || resp.Header.Get("X-Frame-Options") != "DENY" {
		t.Errorf("Sicherheitskopfzeilen fehlen: %v", resp.Header)
	}
}

func TestLocalPageAddsFolderOnlyWithConfirmation(t *testing.T) {
	defer useAppDir(t)()
	allowed, _ := sampleData(t)
	u, srv := newUI(t)
	defer srv.Close()
	if _, err := u.handle("add-folder", []byte(`{"path":"`+jsonPath(allowed)+`","confirmed":false}`)); err == nil {
		t.Fatal("Ordner ohne Haken freigegeben")
	}
	res, err := u.handle("preview", []byte(`{"path":"`+jsonPath(allowed)+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	if n := len(res.(map[string]interface{})["sources"].([]*Source)); n != 1 {
		t.Fatalf("Vorschau fand %d Quellen", n)
	}
	if getSources() != nil && len(getSources()) > 0 {
		for _, s := range getSources() {
			if strings.HasPrefix(s.Path, allowed) {
				t.Fatal("Vorschau hat die freigegebenen Quellen verändert")
			}
		}
	}
	if _, err := u.handle("add-folder", []byte(`{"path":"`+jsonPath(allowed)+`","confirmed":true}`)); err != nil {
		t.Fatal(err)
	}
	if paths := loadConfig().confirmedPaths(); len(paths) != 1 || paths[0] != allowed {
		t.Fatalf("Ordner nicht gespeichert: %v", paths)
	}
	entries := readProtocol(10)
	if len(entries) == 0 || !strings.Contains(entries[0].Text, "Lizenz bestätigt") {
		t.Fatalf("Protokoll fehlt: %+v", entries)
	}
}

func jsonPath(p string) string {
	b, _ := json.Marshal(p)
	return strings.Trim(string(b), `"`)
}

type fakeReplado struct {
	mu      sync.Mutex
	secret  string
	polls   []map[string]interface{}
	results []map[string]interface{}
	calls   []call
	removed bool
	rescan  bool
	update  bool
}

func (f *fakeReplado) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var body map[string]interface{}
	json.NewDecoder(r.Body).Decode(&body)
	authorized := r.Header.Get("Authorization") == "Bearer "+f.secret && f.secret != "" && !f.removed
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/api/connector/v1/login":
		if body["password"] != "richtig" {
			w.WriteHeader(401)
			w.Write([]byte(`{"detail":"E-Mail oder Passwort falsch"}`))
			return
		}
		if body["code"] == nil {
			w.Header().Set("X-Replado-Code", "zwei_faktor")
			w.WriteHeader(401)
			w.Write([]byte(`{"detail":"Code fehlt"}`))
			return
		}
		f.secret = "lc_geheim"
		json.NewEncoder(w).Encode(map[string]interface{}{"connector_id": "abc", "secret": f.secret, "known": []Known{{Name: "Testsoftware", Requires: []string{"KUNDEN"}, Hint: "Kunden"}}})
	case "/api/connector/v1/poll":
		if !authorized {
			w.WriteHeader(401)
			return
		}
		f.polls = append(f.polls, body)
		json.NewEncoder(w).Encode(map[string]interface{}{"calls": f.calls, "rescan": f.rescan, "update": f.update})
		f.calls, f.rescan, f.update = nil, false, false
	case "/api/connector/v1/result":
		if !authorized {
			w.WriteHeader(401)
			return
		}
		f.results = append(f.results, body)
		w.WriteHeader(204)
	case "/api/connector/v1/logout":
		f.secret = ""
		w.WriteHeader(204)
	default:
		w.WriteHeader(404)
	}
}

func TestLoginPollAndAnswerAgainstReplado(t *testing.T) {
	defer useAppDir(t)()
	allowed, other := sampleData(t)
	fake := &fakeReplado{}
	srv := httptest.NewServer(fake)
	defer srv.Close()
	cl := httpClient(5 * time.Second)

	if _, err := login(cl, srv.URL, "admin@firma.de", "falsch", ""); err == nil || !strings.Contains(err.Error(), "Passwort falsch") {
		t.Fatalf("falsches Passwort: %v", err)
	}
	if _, err := login(cl, srv.URL, "admin@firma.de", "richtig", ""); err != errSecondFactor {
		t.Fatalf("Zwei-Faktor nicht erkannt: %v", err)
	}
	c, err := login(cl, srv.URL, "admin@firma.de", "richtig", "123456")
	if err != nil {
		t.Fatal(err)
	}
	if c.Secret != "lc_geheim" || loadConfig().Secret != "lc_geheim" || len(loadConfig().Known) != 1 {
		t.Fatalf("Zugang nicht gespeichert: %+v", loadConfig())
	}
	if st, err := os.Stat(configPath()); err != nil || st.Mode().Perm()&0077 != 0 {
		t.Fatalf("Konfiguration ist für andere lesbar: %v", st.Mode())
	}

	c, _ = updateConfig(func(c *Config) error { return addFolder(c, allowed, true) })
	s := &scanner{}
	s.ensure(c, false)
	var allowedID string
	for _, src := range getSources() {
		allowedID = src.ID
	}
	fake.calls = []call{
		{ID: "1", Tool: "query", Args: json.RawMessage(`{"source":"` + allowedID + `","table":"KUNDEN","limit":5}`)},
		{ID: "2", Tool: "list_sources", Args: json.RawMessage(`{}`)},
		{ID: "3", Tool: "query", Args: json.RawMessage(`{"source":"` + scanOnly([]string{other})[0].ID + `","table":"GEHEIM"}`)},
	}
	if err := pollOnce(cl, cl, c, s); err != nil {
		t.Fatal(err)
	}
	if len(fake.polls) != 1 || fake.polls[0]["sources"] == nil || fake.polls[0]["folders"] == nil {
		t.Fatalf("Quellen und Ordner nicht gemeldet: %+v", fake.polls)
	}
	folders := fake.polls[0]["folders"].([]interface{})
	if folders[0].(map[string]interface{})["confirmation"] != confirmationText {
		t.Fatal("Bestätigungstext fehlt in der Meldung")
	}
	byID := map[string]map[string]interface{}{}
	for _, r := range fake.results {
		byID[r["id"].(string)] = r
	}
	if byID["1"]["result"] == nil || byID["2"]["error"] == nil || byID["3"]["error"] == nil {
		t.Fatalf("Ergebnisse falsch: %+v", fake.results)
	}
	if err := pollOnce(cl, cl, c, s); err != nil {
		t.Fatal(err)
	}
	if _, again := fake.polls[1]["sources"]; again {
		t.Fatal("unveränderte Quellen wurden erneut geschickt")
	}
	fake.mu.Lock()
	fake.rescan = true
	fake.mu.Unlock()
	if err := pollOnce(cl, cl, c, s); err != nil {
		t.Fatal(err)
	}
	if err := pollOnce(cl, cl, c, s); err != nil {
		t.Fatal(err)
	}
	if _, sent := fake.polls[3]["sources"]; !sent {
		t.Fatal("nach dem Neueinlesen wurden die Quellen nicht gemeldet")
	}
	logged := readProtocol(20)
	found := false
	for _, e := range logged {
		if e.Kind == "abfrage" && e.Table == "KUNDEN" && e.Rows == 2 {
			found = true
		}
		if strings.Contains(e.Text+e.Error, "Hans Meier") {
			t.Fatal("Inhalt im Protokoll")
		}
	}
	if !found {
		t.Fatalf("Abfrage fehlt im Protokoll: %+v", logged)
	}

	fake.mu.Lock()
	fake.removed = true
	fake.mu.Unlock()
	err = pollOnce(cl, cl, c, s)
	if ae, ok := err.(*apiError); !ok || ae.status != 401 {
		t.Fatalf("entfernter Connector: %v", err)
	}
	if err := logout(cl); err != nil || loadConfig().Secret != "" {
		t.Fatalf("Abmelden: %v", err)
	}
}

func TestRedirectsAreNotFollowed(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Weiterleitung wurde verfolgt, Zugangsdaten wären fremd gelandet")
	}))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/api/connector/v1/login", http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()
	if _, err := login(httpClient(5*time.Second), redirect.URL, "a@b.de", "pw", ""); err == nil {
		t.Fatal("Weiterleitung als Erfolg gewertet")
	}
}

func TestProtocolRotatesAndReadsNewestFirst(t *testing.T) {
	defer useAppDir(t)()
	for i := 0; i < 3; i++ {
		record(Entry{Kind: "abfrage", Table: "T" + itoa(i)})
	}
	entries := readProtocol(2)
	if len(entries) != 2 || entries[0].Table != "T2" {
		t.Fatalf("Reihenfolge falsch: %+v", entries)
	}
	big := strings.Repeat("x", maxProtocolBytes+1)
	record(Entry{Kind: "groß", Text: big})
	record(Entry{Kind: "neu"})
	if _, err := os.Stat(protocolPath() + ".alt"); err != nil {
		t.Fatal("Protokoll wurde nicht gewechselt")
	}
	if readProtocol(1)[0].Kind != "neu" {
		t.Fatal("neuester Eintrag fehlt")
	}
}

func TestSearchFindsWordsAcrossColumnsAndSpellings(t *testing.T) {
	row := []interface{}{"Tabak-Börse", "Gräser", "Hauptstraße 5", "Musterstadt", 4711.0}
	for _, text := range []string{"Tabakbörse Gräser", "tabakboerse graeser", "Gräser", "TABAK börse", "Graeser Musterstadt", "hauptstrasse"} {
		if !rowMatches(row, searchWords(text)) {
			t.Errorf("%q nicht gefunden", text)
		}
	}
	for _, text := range []string{"Tabakbörse Meier", "4711", "Gräserstraße"} {
		if rowMatches(row, searchWords(text)) {
			t.Errorf("%q fälschlich gefunden", text)
		}
	}
	if !rowMatches([]interface{}{"info@tabak-graeser.de"}, searchWords("info@tabak-graeser.de")) {
		t.Error("Mailadresse nicht gefunden")
	}
	if len(searchWords("  - , ")) != 0 {
		t.Error("Satzzeichen allein sind kein Suchtext")
	}
}

func TestUpdateHintFromRepladoStartsOneCheck(t *testing.T) {
	defer useAppDir(t)()
	fake := &fakeReplado{secret: "lc_geheim", update: true}
	srv := httptest.NewServer(fake)
	defer srv.Close()
	started := make(chan struct{}, 4)
	release := make(chan struct{})
	old := runUpdateCheck
	runUpdateCheck = func() {
		started <- struct{}{}
		<-release
	}
	defer func() { runUpdateCheck = old }()
	cl := httpClient(5 * time.Second)
	c := &Config{Server: srv.URL, Secret: "lc_geheim"}
	if err := pollOnce(cl, cl, c, &scanner{}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("Hinweis von Replado hat keine Update-Prüfung gestartet")
	}
	if checkForUpdateOnce() {
		t.Fatal("zweite Prüfung lief trotz laufender Prüfung")
	}
	close(release)
	for i := 0; i < 100 && atomic.LoadInt32(&updateRunning) != 0; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if err := pollOnce(cl, cl, c, &scanner{}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
		t.Fatal("ohne Hinweis wurde geprüft")
	case <-time.After(200 * time.Millisecond):
	}
}
